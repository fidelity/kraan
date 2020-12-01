// Package metrics contains definition of metrics and helper functions related to metrics

//go:generate mockgen -destination=../mocks/metrics/mockmetrics.go -package=mocks -source=metrics.go . Metrics
package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

type Metrics interface {
	Init()
	RecordCondition(obj runtime.Object, condition metav1.Condition, deleted bool)
	RecordDuration(obj runtime.Object, start time.Time)
}

type metricsData struct {
	Metrics
	durationHistogram *prometheus.HistogramVec
	conditionGauge    *prometheus.GaugeVec
}

const (
	ConditionDeleted = "Deleted"
)

// NewMetrics returns a metrics interface
func NewMetrics() Metrics {
	m := &metricsData{}
	m.Init()
	return m
}

// Init initialized the metrics
func (m *metricsData) Init() {
	m.conditionGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "reconcile_condition",
			Help: "The current condition status of a GitOps Toolkit resource reconciliation.",
		},
		[]string{"kind", "name", "namespace", "type", "status"},
	)
	ctlmetrics.Registry.MustRegister(m.conditionGauge)

	m.durationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "reconcile_duration_seconds",
			Help:    "The duration in seconds of a GitOps Toolkit resource reconciliation.",
			Buckets: prometheus.ExponentialBuckets(10e-9, 10, 10),
		},
		[]string{"kind", "name", "namespace"},
	)
	ctlmetrics.Registry.MustRegister(m.durationHistogram)
}

// RecordCondition records condition metrics
func (m *metricsData) RecordCondition(obj runtime.Object, condition metav1.Condition, deleted bool) {
	for _, status := range []string{string(corev1.ConditionTrue), string(corev1.ConditionFalse), string(corev1.ConditionUnknown), ConditionDeleted} {
		var value float64
		if deleted {
			if status == ConditionDeleted {
				value = 1
			}
		} else {
			if status == string(condition.Status) {
				value = 1
			}
		}
		args := append(getObjKindNamespaceName(obj), condition.Type, status)
		m.conditionGauge.WithLabelValues(args...).Set(value)
	}
}

// RecordDuration records duration metrics
func (m *metricsData) RecordDuration(obj runtime.Object, start time.Time) {
	m.durationHistogram.WithLabelValues(getObjKindNamespaceName(obj)...).Observe(time.Since(start).Seconds())
}

func getObjKindNamespaceName(obj runtime.Object) []string {
	mobj, ok := (obj).(metav1.Object)
	if !ok {
		return []string{"unavailable", "unavailable", "unavailable"}
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	return []string{fmt.Sprintf("%s.%s", gvk.Kind, gvk.Group), mobj.GetNamespace(), mobj.GetName()}
}

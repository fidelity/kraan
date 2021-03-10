// Code generated by MockGen. DO NOT EDIT.
// Source: metrics.go

// Package mocks is a generated GoMock package.
package mocks

import (
	gomock "github.com/golang/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	reflect "reflect"
	time "time"
)

// MockMetrics is a mock of Metrics interface
type MockMetrics struct {
	ctrl     *gomock.Controller
	recorder *MockMetricsMockRecorder
}

// MockMetricsMockRecorder is the mock recorder for MockMetrics
type MockMetricsMockRecorder struct {
	mock *MockMetrics
}

// NewMockMetrics creates a new mock instance
func NewMockMetrics(ctrl *gomock.Controller) *MockMetrics {
	mock := &MockMetrics{ctrl: ctrl}
	mock.recorder = &MockMetricsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockMetrics) EXPECT() *MockMetricsMockRecorder {
	return m.recorder
}

// Init mocks base method
func (m *MockMetrics) Init() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Init")
}

// Init indicates an expected call of Init
func (mr *MockMetricsMockRecorder) Init() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockMetrics)(nil).Init))
}

// RecordCondition mocks base method
func (m *MockMetrics) RecordCondition(obj runtime.Object, condition v1.Condition, deleted bool) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RecordCondition", obj, condition, deleted)
}

// RecordCondition indicates an expected call of RecordCondition
func (mr *MockMetricsMockRecorder) RecordCondition(obj, condition, deleted interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordCondition", reflect.TypeOf((*MockMetrics)(nil).RecordCondition), obj, condition, deleted)
}

// RecordDuration mocks base method
func (m *MockMetrics) RecordDuration(obj runtime.Object, start time.Time) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RecordDuration", obj, start)
}

// RecordDuration indicates an expected call of RecordDuration
func (mr *MockMetricsMockRecorder) RecordDuration(obj, start interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecordDuration", reflect.TypeOf((*MockMetrics)(nil).RecordDuration), obj, start)
}

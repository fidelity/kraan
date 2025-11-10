package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	helmctlv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kraanv1alpha1 "github.com/fidelity/kraan/api/v1alpha1"
	"github.com/fidelity/kraan/pkg/apply"
	"github.com/fidelity/kraan/pkg/layers"
)

//nolint:gocognit,gocyclo,funlen // Test function complexity and length acceptable for comprehensive test coverage
func TestSuspendHelmReleasesOnHold(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := kraanv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add kraan scheme: %v", err)
	}
	if err := helmctlv2.AddToScheme(scheme); err != nil {
		t.Fatalf("add helm v2 scheme: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name             string
		hrs              map[string]*helmctlv2.HelmRelease
		preSuspend       map[string]bool // set initial Spec.Suspend for HRs
		expectSuspended  map[string]bool
		expectAnno       map[string]bool
		expectEvent      bool
		expectEventCount int
	}{
		{
			name: "suspends all not suspended",
			hrs: map[string]*helmctlv2.HelmRelease{
				"ns1/hr1": {ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "hr1"}},
				"ns2/hr2": {ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "hr2"}},
			},
			preSuspend:       map[string]bool{"ns1/hr1": false, "ns2/hr2": false},
			expectSuspended:  map[string]bool{"ns1/hr1": true, "ns2/hr2": true},
			expectAnno:       map[string]bool{"ns1/hr1": true, "ns2/hr2": true},
			expectEvent:      true,
			expectEventCount: 2,
		},
		{
			name: "skips already suspended",
			hrs: map[string]*helmctlv2.HelmRelease{
				"ns1/hr1": {ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "hr1"}},
				"ns2/hr2": {ObjectMeta: metav1.ObjectMeta{Namespace: "ns2", Name: "hr2"}},
			},
			preSuspend:       map[string]bool{"ns1/hr1": true, "ns2/hr2": false},
			expectSuspended:  map[string]bool{"ns1/hr1": true, "ns2/hr2": true},
			expectAnno:       map[string]bool{"ns1/hr1": false, "ns2/hr2": true},
			expectEvent:      true,
			expectEventCount: 1,
		},
		{
			name:             "no helmreleases",
			hrs:              map[string]*helmctlv2.HelmRelease{},
			preSuspend:       map[string]bool{},
			expectSuspended:  map[string]bool{},
			expectAnno:       map[string]bool{},
			expectEvent:      false,
			expectEventCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh objects
			al := &kraanv1alpha1.AddonsLayer{
				ObjectMeta: metav1.ObjectMeta{Name: "layer-hold"},
				Spec:       kraanv1alpha1.AddonsLayerSpec{Version: "v1", Hold: true},
			}

			objs := []client.Object{al}
			// set owner refs and pre-suspend
			for _, hr := range tc.hrs {
				if err := controllerutil.SetControllerReference(al, hr, scheme); err != nil {
					t.Fatalf("ownerref: %v", err)
				}
				if tc.preSuspend[fmt.Sprintf("%s/%s", hr.Namespace, hr.Name)] {
					hr.Spec.Suspend = true
				}
				objs = append(objs, hr.DeepCopy())
			}

			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
			rec := record.NewFakeRecorder(10)
			l := layers.CreateLayer(ctx, c, nil, logr.Discard(), rec, scheme, al)

			r := &AddonsLayerReconciler{
				Client:   c,
				Scheme:   scheme,
				Log:      logr.Discard(),
				Applier:  apply.FakeApplier{Hrs: tc.hrs},
				Context:  ctx,
				Recorder: rec,
			}

			if _, err := r.processAddonLayer(l); err != nil {
				t.Fatalf("processAddonLayer: %v", err)
			}

			// Verify expectations per HR
			for key, hr := range tc.hrs {
				got := &helmctlv2.HelmRelease{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(hr), got); err != nil {
					t.Fatalf("get %s: %v", key, err)
				}
				if got.Spec.Suspend != tc.expectSuspended[key] {
					t.Fatalf("%s suspend=%v, want %v", key, got.Spec.Suspend, tc.expectSuspended[key])
				}
				annoSet := got.Annotations["kraan/suspended-by-hold"] == "true"
				if annoSet != tc.expectAnno[key] {
					t.Fatalf("%s annotation set=%v, want %v", key, annoSet, tc.expectAnno[key])
				}
			}

			// Collect any events
			events := []string{}
			for {
				select {
				case e := <-rec.Events:
					events = append(events, e)
				default:
					goto done
				}
			}
		done:
			hasSuspended := false
			suspendedMsgOK := false
			for _, e := range events {
				if strings.HasPrefix(e, "Normal Suspended") {
					hasSuspended = true
					if strings.Contains(e, fmt.Sprintf("%d HelmRelease(s)", tc.expectEventCount)) {
						suspendedMsgOK = true
					}
				}
			}

			if tc.expectEvent && !hasSuspended {
				t.Fatalf("expected Suspended event, got none: %+v", events)
			}
			if !tc.expectEvent && hasSuspended {
				t.Fatalf("unexpected Suspended event: %+v", events)
			}
			if tc.expectEvent && !suspendedMsgOK {
				t.Fatalf("Suspended event message does not contain expected count: %+v", events)
			}
		})
	}
}

// Code generated by MockGen. DO NOT EDIT.
// Source: layerApplier.go

package apply

import (
	context "context"
	layers "github.com/fidelity/kraan/pkg/layers"
	v1 "github.com/fluxcd/helm-operator/pkg/apis/helm.fluxcd.io/v1"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockLayerApplier is a mock of LayerApplier interface
type MockLayerApplier struct {
	ctrl     *gomock.Controller
	recorder *MockLayerApplierMockRecorder
}

// MockLayerApplierMockRecorder is the mock recorder for MockLayerApplier
type MockLayerApplierMockRecorder struct {
	mock *MockLayerApplier
}

// NewMockLayerApplier creates a new mock instance
func NewMockLayerApplier(ctrl *gomock.Controller) *MockLayerApplier {
	mock := &MockLayerApplier{ctrl: ctrl}
	mock.recorder = &MockLayerApplierMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockLayerApplier) EXPECT() *MockLayerApplierMockRecorder {
	return _m.recorder
}

// Apply mocks base method
func (_m *MockLayerApplier) Apply(ctx context.Context, layer layers.Layer) error {
	ret := _m.ctrl.Call(_m, "Apply", ctx, layer)
	ret0, _ := ret[0].(error)
	return ret0
}

// Apply indicates an expected call of Apply
func (_mr *MockLayerApplierMockRecorder) Apply(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "Apply", reflect.TypeOf((*MockLayerApplier)(nil).Apply), arg0, arg1)
}

// Prune mocks base method
func (_m *MockLayerApplier) Prune(ctx context.Context, layer layers.Layer, pruneHrs []*v1.HelmRelease) error {
	ret := _m.ctrl.Call(_m, "Prune", ctx, layer, pruneHrs)
	ret0, _ := ret[0].(error)
	return ret0
}

// Prune indicates an expected call of Prune
func (_mr *MockLayerApplierMockRecorder) Prune(arg0, arg1, arg2 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "Prune", reflect.TypeOf((*MockLayerApplier)(nil).Prune), arg0, arg1, arg2)
}

// PruneIsRequired mocks base method
func (_m *MockLayerApplier) PruneIsRequired(ctx context.Context, layer layers.Layer) (bool, []*v1.HelmRelease, error) {
	ret := _m.ctrl.Call(_m, "PruneIsRequired", ctx, layer)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].([]*v1.HelmRelease)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// PruneIsRequired indicates an expected call of PruneIsRequired
func (_mr *MockLayerApplierMockRecorder) PruneIsRequired(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "PruneIsRequired", reflect.TypeOf((*MockLayerApplier)(nil).PruneIsRequired), arg0, arg1)
}

// ApplyIsRequired mocks base method
func (_m *MockLayerApplier) ApplyIsRequired(ctx context.Context, layer layers.Layer) (bool, error) {
	ret := _m.ctrl.Call(_m, "ApplyIsRequired", ctx, layer)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplyIsRequired indicates an expected call of ApplyIsRequired
func (_mr *MockLayerApplierMockRecorder) ApplyIsRequired(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ApplyIsRequired", reflect.TypeOf((*MockLayerApplier)(nil).ApplyIsRequired), arg0, arg1)
}

// ApplyWasSuccessful mocks base method
func (_m *MockLayerApplier) ApplyWasSuccessful(ctx context.Context, layer layers.Layer) (bool, error) {
	ret := _m.ctrl.Call(_m, "ApplyWasSuccessful", ctx, layer)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplyWasSuccessful indicates an expected call of ApplyWasSuccessful
func (_mr *MockLayerApplierMockRecorder) ApplyWasSuccessful(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "ApplyWasSuccessful", reflect.TypeOf((*MockLayerApplier)(nil).ApplyWasSuccessful), arg0, arg1)
}

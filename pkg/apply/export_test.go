package apply

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-logr/logr"

	"github.com/fidelity/kraan/pkg/internal/kubectl"
)

func SetNewKubectlFunc(kubectlFunc func(logger logr.Logger) (kubectl.Kubectl, error)) {
	newKubectlFunc = kubectlFunc
}

var (
	AddOwnerRefs  = LayerApplier.addOwnerRefs
	OrphanLabel   = LayerApplier.orphanLabel
	OrphanedLabel = orphanedLabel
	OwnerLabel    = ownerLabel
	LayerOwner    = layerOwner
	ChangeOwner   = changeOwner
	GetTimestamp  = getTimestamp
	LabelValue    = labelValue
	GetObjLabel   = getObjLabel
)

func GetField(t *testing.T, obj interface{}, fieldName string) interface{} {
	o, ok := obj.(KubectlLayerApplier)
	if !ok {
		t.Fatalf("failed to cast to KubectlLayerApplier")

		return nil
	}

	rs := reflect.ValueOf(&o).Elem()
	typ := reflect.TypeOf(o)

	for i := 0; i < rs.NumField(); i++ {
		rf := rs.Field(i)
		fieldValue := reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()

		if typ.Field(i).Name == fieldName {
			return fieldValue.Interface()
		}
	}
	t.Fatalf("failed to find field: %s in stuct config", fieldName)

	return nil
}

package layers

import (
	"fmt"
	"os"
	"reflect"
	"unsafe"
)

func GetField(layer Layer, fieldName string) (*reflect.Value, error) {
	obj, ok := layer.(*kraanLayer)
	if !ok {
		return nil, fmt.Errorf("failed to cast layer interface to *kraanLayer")
	}

	rs := reflect.ValueOf(obj).Elem()
	typ := reflect.TypeOf(*obj)
	for i := 0; i < rs.NumField(); i++ {
		rf := rs.Field(i)
		fieldValue := reflect.NewAt(rf.Type(), unsafe.Pointer(rf.UnsafeAddr())).Elem()
		fmt.Fprintf(os.Stderr, "Type: %s, %s=%v\n", typ.Field(i).Type, typ.Field(i).Name, fieldValue)
		if typ.Field(i).Name == fieldName {
			return &fieldValue, nil
		}
	}
	return nil, fmt.Errorf("failed to find field %s in stuct kraanLayer", fieldName)
}

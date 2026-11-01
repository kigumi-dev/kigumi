package driver

import (
	"reflect"
	"testing"

	mirreach "kigumi/internal/compiler/mir/reach"
)

// The record copy lists its fields by hand, so this fills every field reach has and expects each one back under its name.
func TestCapabilityRecordKeepsEveryField(t *testing.T) {
	var r mirreach.CapabilityReason
	fillDistinct(reflect.ValueOf(&r).Elem(), "r")
	sameFields(t, "CapabilityReason", reflect.ValueOf(r), reflect.ValueOf(capabilityRecord(r)))

	empty := capabilityRecord(mirreach.CapabilityReason{})
	if empty.Path != nil || empty.Sites != nil {
		t.Fatalf("nil slices must stay nil for the sidecar JSON: %+v", empty)
	}
}

func fillDistinct(v reflect.Value, name string) {
	switch v.Kind() {
	case reflect.String:
		v.SetString(name)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int:
		v.SetInt(int64(len(name)))
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillDistinct(v.Index(0), name+"[0]")
	case reflect.Struct:
		for i := range v.NumField() {
			fillDistinct(v.Field(i), name+"."+v.Type().Field(i).Name)
		}
	default:
		panic("fillDistinct: no filler for " + v.Kind().String())
	}
}

func sameFields(t *testing.T, path string, src, dst reflect.Value) {
	t.Helper()
	if src.Kind() != dst.Kind() {
		t.Fatalf("%s: %s in reach, %s in the sidecar", path, src.Kind(), dst.Kind())
	}
	switch src.Kind() {
	case reflect.Struct:
		if src.NumField() != dst.NumField() {
			t.Fatalf("%s: %d fields in reach, %d in the sidecar", path, src.NumField(), dst.NumField())
		}
		for i := range src.NumField() {
			name := src.Type().Field(i).Name
			f := dst.FieldByName(name)
			if !f.IsValid() {
				t.Fatalf("%s.%s has no sidecar field", path, name)
			}
			sameFields(t, path+"."+name, src.Field(i), f)
		}
	case reflect.Slice:
		if src.Len() != dst.Len() {
			t.Fatalf("%s: length %d, sidecar %d", path, src.Len(), dst.Len())
		}
		for i := range src.Len() {
			sameFields(t, path+"[]", src.Index(i), dst.Index(i))
		}
	default:
		if src.Interface() != dst.Interface() {
			t.Fatalf("%s: %v copied as %v", path, src.Interface(), dst.Interface())
		}
	}
}

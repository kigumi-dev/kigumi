package vm

import (
	"testing"
	"unsafe"
)

func TestObjCoreStaysSmall(t *testing.T) {
	if n := unsafe.Sizeof(obj{}); n > 64 {
		t.Errorf("obj is %d bytes, want at most 64", n)
	}
}

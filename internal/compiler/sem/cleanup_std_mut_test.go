package sem_test

import (
	"slices"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// A new entry here must also be classified in stdReplacesReceiver when it can replace the receiver's content.
var bodilessStdMutSelf = []string{
	"alloc.Box.set", "alloc.Box.withMut",
	"array.Array.growBy", "array.Array.push", "array.Array.pushCopy", "array.Array.takeAt", "array.Array.tryInsertAt", "array.Array.withMut",
	"prelude.ByteBuffer.copyFrom", "prelude.ByteBuffer.copyFromString", "prelude.ByteBuffer.set",
	"prelude.FixedArray.withMut", "prelude.Option.take",
}

func TestBodilessStdMutSelfClassified(t *testing.T) {
	r := sem.CheckWith(&sem.Module{Packages: semfixture.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	var got []string
	for id := range r.Entities {
		e := &r.Entities[id]
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 {
			continue
		}
		info := r.Fn(sem.EntityID(id))
		if info.Recv != sem.RecvMut || info.Body != 0 {
			continue
		}
		key := strings.TrimPrefix(r.Packages[e.Pkg].Path, "std/")
		if info.Owner != 0 {
			key += "." + r.Entities[info.Owner].Name
		}
		got = append(got, key+"."+e.Name)
	}
	slices.Sort(got)
	if !slices.Equal(got, bodilessStdMutSelf) {
		t.Errorf("bodiless std mut self methods changed:\n got %q\nwant %q", got, bodilessStdMutSelf)
	}
}

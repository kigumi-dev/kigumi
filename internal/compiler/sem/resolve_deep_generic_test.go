package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

func TestDeepGenericInferenceResolvesEachLevelOnce(t *testing.T) {
	misses := map[int]int{}
	for _, depth := range []int{600, 2400} {
		src := "fn wrap[T](x: T) -> Option[T] { Some(x) }\nfn work() -> Int {\nlet v = " + strings.Repeat("wrap(", depth) + "1" + strings.Repeat(")", depth) + "\n0\n}\nprint \"${work()}\"\n"
		r := sem.CheckWith(buildModule(t, fixture{sections: []section{{name: "src", body: src}}}), schema.GenerateDerive)
		if r.HasErrors() {
			t.Fatal(r.Render())
		}
		misses[depth] = r.ResolveMisses()
		t.Logf("depth %d: %d uncached resolutions", depth, misses[depth])
	}
	if grew := misses[2400] - misses[600]; grew <= 0 || grew > 16*1800 {
		t.Fatalf("1800 more nested calls cost %d uncached resolutions, want at most %d", grew, 16*1800)
	}
}

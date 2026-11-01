package driver_test

import (
	"runtime"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// compileWork counts what analyzing a program costs instead of timing it.
type compileWork struct {
	bytes, insts, misses float64
}

func compileWorkOf(t *testing.T, src string) compileWork {
	t.Helper()
	m := generatedModule(t, src)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	a, err := driver.Analyze(m)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if a.Prog == nil {
		t.Fatal(a.Res.Render())
	}
	w := compileWork{bytes: float64(after.TotalAlloc - before.TotalAlloc), misses: float64(a.Res.ResolveMisses())}
	for _, f := range a.Prog.AllFuncs() {
		for _, b := range f.Blocks {
			w.insts += float64(len(b.Insts))
		}
	}
	return w
}

// growthLimit caps each measure's growth over the second doubling against the first: 2 is linear, 4 quadratic, 0 unchecked.
type growthLimit struct {
	bytes, insts, misses float64
}

// At n/16, n/8 and n/4 the quadratic costs these tests guard against already dominate.
func requireGrowth(t *testing.T, gen func(int) string, n int, limit growthLimit) {
	t.Helper()
	w := [3]compileWork{compileWorkOf(t, gen(n/16)), compileWorkOf(t, gen(n/8)), compileWorkOf(t, gen(n/4))}
	check := func(name string, max float64, at func(compileWork) float64) {
		first, second := at(w[1])-at(w[0]), at(w[2])-at(w[1])
		if max == 0 {
			return
		}
		if first <= 0 || second > max*first {
			t.Errorf("%s: the second doubling added %.0f against %.0f for the first, limit %.1fx", name, second, first, max)
		}
		t.Logf("%s grew %.2fx over the second doubling", name, second/first)
	}
	check("allocated bytes", limit.bytes, func(c compileWork) float64 { return c.bytes })
	check("MIR instructions", limit.insts, func(c compileWork) float64 { return c.insts })
	check("uncached resolutions", limit.misses, func(c compileWork) float64 { return c.misses })
	if t.Failed() {
		t.FailNow()
	}
}

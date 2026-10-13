package driver_test

import (
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"kigumi/internal/driver"
	"kigumi/internal/mir"
)

var mutantBases = []string{
	"adt", "closures", "async", "async_method", "direct_aggregate", "direct_closure",
	"direct_fixed_array", "fixed_array", "generic_direct_methods", "generics",
	"mut_self_dispatch", "iface_static", "direct_adt", "conform_generic_method_dispatch",
}

func fixtureFile(base string) string { return base + ".txtar" }

const (
	defaultMutantsPerBase = 600
	defaultMaxFindings    = 4
)

func analyzeFixture(t testing.TB, name string) *mir.Program {
	t.Helper()
	root, _, _, _ := loadRunFixture(t, "../../testdata/run/"+name)
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}
	return a.Prog
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil {
		return v
	}
	return def
}

type sweepStats struct {
	mutants, accepted int
	rejected          map[string]int
	vm                map[vmOutcome]int
}

func checkMutant(base *mir.Program, mu *mutator, seed int64, st *sweepStats, verbose func(string, ...any)) (what, finding string) {
	p, what := mutantOf(base, mu, seed)
	return what, checkProgram(p, st, verbose)
}

func mutantOf(base *mir.Program, mu *mutator, seed int64) (*mir.Program, string) {
	p := cloneProgram(base)
	return p, mu.mutate(p, rand.New(rand.NewSource(seed)))
}

func checkProgram(p *mir.Program, st *sweepStats, verbose func(string, ...any)) (finding string) {
	st.mutants++
	var err error
	if f := guard("verifier", func() { err = p.Verify() }); f != "" {
		return f
	}
	if err != nil {
		rule := "?"
		if ve, ok := err.(*mir.Error); ok {
			rule = ve.Rule
		}
		st.rejected[rule]++
		verbose("rejected: %v", err)
		return ""
	}
	st.accepted++
	if f := emitMutant(p); f != "" {
		return f
	}
	out, f := runMutant(p)
	if f == "" {
		st.vm[out]++
	}
	verbose("accepted; vm: %s", out)
	return f
}

func (st *sweepStats) String() string {
	var rules []string
	for r, n := range st.rejected {
		rules = append(rules, fmt.Sprintf("%s=%d", r, n))
	}
	sort.Strings(rules)
	var outs []string
	for o, n := range st.vm {
		outs = append(outs, fmt.Sprintf("%s=%d", o, n))
	}
	sort.Strings(outs)
	return fmt.Sprintf("%d mutants, %d accepted (vm: %s), rejected by %s", st.mutants, st.accepted, strings.Join(outs, ", "), strings.Join(rules, ", "))
}

// TestMutatedMIR sweeps a fixed count of seeds per base; an accepted mutant must
// not crash llgen or the VM. Env KIGUMI_MIR_FUZZ_: COUNT, SECONDS, FROM, MAXFINDINGS, IR, REPLAY.
func TestMutatedMIR(t *testing.T) {
	if r := os.Getenv("KIGUMI_MIR_FUZZ_REPLAY"); r != "" {
		replayMutant(t, r)
		return
	}
	count := envInt("KIGUMI_MIR_FUZZ_COUNT", defaultMutantsPerBase)
	seconds := envInt("KIGUMI_MIR_FUZZ_SECONDS", 0)
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	from := int64(envInt("KIGUMI_MIR_FUZZ_FROM", 0))
	maxFindings := envInt("KIGUMI_MIR_FUZZ_MAXFINDINGS", defaultMaxFindings)
	var mu sync.Mutex
	for _, name := range mutantBases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base := analyzeFixture(t, fixtureFile(name))
			m := newMutator(base)
			st := &sweepStats{rejected: map[string]int{}, vm: map[vmOutcome]int{}}
			findings := 0
			for seed := from; findings < maxFindings && (st.mutants < count || time.Now().Before(deadline)); seed++ {
				if what, finding := checkMutant(base, m, seed, st, func(string, ...any) {}); finding != "" {
					findings++
					t.Errorf("seed %d (%s): verifier accepted MIR the back end could not take: %s\nreplay: KIGUMI_MIR_FUZZ_REPLAY=%s:%d go test ./internal/driver/ -run TestMutatedMIR -v", seed, what, finding, name, seed)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			t.Logf("%s: %s", name, st)
		})
	}
}

func replayMutant(t *testing.T, spec string) {
	name, seedText, _ := strings.Cut(spec, ":")
	seed, err := strconv.ParseInt(seedText, 10, 64)
	if err != nil {
		t.Fatalf("KIGUMI_MIR_FUZZ_REPLAY wants <base>:<seed>, got %q", spec)
	}
	debug.SetTraceback("all")
	base := analyzeFixture(t, fixtureFile(name))
	st := &sweepStats{rejected: map[string]int{}, vm: map[vmOutcome]int{}}
	p, what := mutantOf(base, newMutator(base), seed)
	t.Logf("mutant: %s", what)
	for _, d := range changedFuncs(base, p) {
		t.Logf("%s", d)
	}
	for _, d := range changedLocals(base, p) {
		t.Logf("%s", d)
	}
	finding := checkProgram(p, st, func(f string, a ...any) { t.Logf(f, a...) })
	if finding != "" {
		t.Errorf("%s", finding)
	}
}

func FuzzMutatedMIR(f *testing.F) {
	bases := make([]*mir.Program, len(mutantBases))
	muts := make([]*mutator, len(mutantBases))
	for i, name := range mutantBases {
		bases[i] = analyzeFixture(f, fixtureFile(name))
		muts[i] = newMutator(bases[i])
	}
	for seed := int64(0); seed < 8; seed++ {
		f.Add(uint8(seed), seed)
	}
	f.Fuzz(func(t *testing.T, which uint8, seed int64) {
		i := int(which) % len(mutantBases)
		st := &sweepStats{rejected: map[string]int{}, vm: map[vmOutcome]int{}}
		if what, finding := checkMutant(bases[i], muts[i], seed, st, func(string, ...any) {}); finding != "" {
			t.Fatalf("%s seed %d (%s): %s", mutantBases[i], seed, what, finding)
		}
	})
}

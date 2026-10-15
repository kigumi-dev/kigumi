package driver_test

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

type sweepFixture struct {
	name  string
	files map[string]string
	entry string
	test  bool
	// keepClean undoes a mutation whose whole check has errors, so the walk spends its steps on programs that check.
	keepClean bool
}

var sweepOptions = map[string]bool{"entry": true, "mir": true, "test": true}

func readSweepFixture(path string) (sweepFixture, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return sweepFixture{}, false
	}
	fx := sweepFixture{name: strings.TrimSuffix(filepath.Base(path), ".txtar"), files: map[string]string{}}
	var cur string
	var body strings.Builder
	flush := func() {
		if cur == "" {
			return
		}
		switch {
		case cur == "module":
			for _, l := range strings.Split(strings.TrimSpace(body.String()), "\n") {
				k, v, _ := strings.Cut(l, ":")
				k, v = strings.TrimSpace(k), strings.TrimSpace(v)
				if !sweepOptions[k] {
					cur = "!"
				}
				if k == "entry" {
					fx.entry = v
				}
			}
		case cur == "src":
			fx.files["main/main.kg"] = body.String()
		case strings.HasSuffix(cur, ".kg"):
			fx.files[cur] = body.String()
		}
		body.Reset()
	}
	for _, l := range strings.SplitAfter(string(raw), "\n") {
		if t := strings.TrimRight(l, "\n"); strings.HasPrefix(t, "-- ") && strings.HasSuffix(t, " --") {
			flush()
			if cur == "!" {
				return fx, false
			}
			cur = strings.TrimSuffix(strings.TrimPrefix(t, "-- "), " --")
			if cur == "expect" {
				cur = ""
				break
			}
			continue
		}
		if cur != "" {
			body.WriteString(l)
		}
	}
	flush()
	if cur == "!" || len(fx.files) == 0 {
		return fx, false
	}
	if _, ok := fx.files["main/main.kg"]; ok && fx.entry == "" {
		fx.entry = "main/main.kg"
	}
	for name := range fx.files {
		if strings.HasPrefix(name, "std/") {
			return fx, false
		}
		if strings.HasSuffix(name, "_test.kg") {
			fx.test = true
		}
	}
	return fx, true
}

func writeSweep(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ageTree(t, root)
}

func sweepLoad(t *testing.T, fx sweepFixture, root, cache string) *driver.Module {
	t.Helper()
	opts := driver.LoadOptions{StdRoot: stdRoot(t), Test: fx.test, Entry: fx.entry}
	if cache != "" {
		opts.PackageCache, opts.DeferParse, opts.UnitCheck = cache, true, true
	}
	m, err := driver.LoadModule(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

var sweepWord = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*|[0-9]+`)

func sweepMutate(rng *rand.Rand, src string) string {
	lines := strings.Split(src, "\n")
	switch i := rng.Intn(len(lines)); rng.Intn(4) {
	case 0:
		return strings.Join(append(lines[:i:i], lines[i+1:]...), "\n")
	case 1:
		return strings.Join(append(lines[:i+1:i+1], lines[i:]...), "\n")
	case 2:
		locs := sweepWord.FindAllStringIndex(src, -1)
		if len(locs) < 2 {
			return src
		}
		a, b := locs[rng.Intn(len(locs))], locs[rng.Intn(len(locs))]
		return src[:a[0]] + src[b[0]:b[1]] + src[a[1]:]
	default:
		for _, kw := range []string{"noalloc ", "pure ", "pub ", "mut "} {
			if j := strings.Index(src, kw); j >= 0 && rng.Intn(2) == 0 {
				return src[:j] + src[j+len(kw):]
			}
		}
		return src + "\nfn sweepProbe() -> Int {\n    1\n}\n"
	}
}

func parsesClean(name, src string) bool {
	return !syntax.Parse(token.NewFile(name, []byte(src))).HasErrors()
}

func sweepSeed(name string, k int) int64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s/%d", name, k)
	return int64(h.Sum64())
}

type sweepTotals struct{ fixtures, multi, runs, replays, fallbacks, lazy, lazyRetries int }

func sweepFixtureRun(t *testing.T, fx sweepFixture, mutations int, tot *sweepTotals) {
	root, cache := t.TempDir(), t.TempDir()
	files := map[string]string{}
	for k, v := range fx.files {
		files[k] = v
	}
	writeSweep(t, root, files)
	var names []string
	for n := range files {
		if n != "mod.kg" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	packages := map[string]bool{}
	for _, n := range names {
		packages[filepath.Dir(n)] = true
	}
	tot.fixtures++
	if len(packages) > 1 {
		tot.multi++
	}
	var prev, prevTarget string
	for step := 0; step <= mutations; step++ {
		if step > 0 {
			rng := rand.New(rand.NewSource(sweepSeed(fx.name, step)))
			target := names[rng.Intn(len(names))]
			next := sweepMutate(rng, files[target])
			if next == files[target] || !parsesClean(target, next) {
				continue
			}
			prev = files[target]
			files[target], prevTarget = next, target
			writeSweep(t, root, map[string]string{target: next})
		}
		wm := sweepLoad(t, fx, root, "")
		wres, werr := driver.Check(wm)
		um := sweepLoad(t, fx, root, cache)
		ures, uerr := driver.CheckUnits(um)
		if (werr == nil) != (uerr == nil) || (werr != nil && werr.Error() != uerr.Error()) {
			t.Fatalf("%s step %d: whole check error %v, unit check error %v\n%s", fx.name, step, werr, uerr, sweepDiff(fx.files, files))
		}
		if werr != nil {
			continue
		}
		if ures.Render() != wres.Render() || ures.HasErrors() != wres.HasErrors() {
			t.Fatalf("%s step %d: unit check differs from the whole check\n--- unit\n%s--- whole\n%s", fx.name, step, ures.Render(), wres.Render())
		}
		if fx.keepClean && wres.HasErrors() && prevTarget != "" {
			files[prevTarget] = prev
			writeSweep(t, root, map[string]string{prevTarget: prev})
		}
		st := um.UnitStats()
		tot.runs += st.Runs
		tot.fallbacks += st.Fallbacks
		tot.lazy += st.Lazy
		tot.lazyRetries += st.LazyRetries
		for _, n := range st.Replayed {
			tot.replays += n
		}
	}
}

func TestUnitSweepSemFixtures(t *testing.T) {
	paths, _ := filepath.Glob("../../testdata/sem/*.txtar")
	var tot sweepTotals
	singles := 0
	for _, p := range paths {
		fx, ok := readSweepFixture(p)
		if !ok {
			continue
		}
		if len(fx.files) == 1 {
			if singles++; singles%unitSweepSingleStride != 0 {
				continue
			}
		}
		t.Run(fx.name, func(t *testing.T) { sweepFixtureRun(t, fx, unitSweepMutations, &tot) })
	}
	t.Logf("fixtures=%d multi-package=%d semantic runs=%d replayed packages=%d fallbacks=%d lazy runs=%d lazy retries=%d", tot.fixtures, tot.multi, tot.runs, tot.replays, tot.fallbacks, tot.lazy, tot.lazyRetries)
}

const (
	unitSweepMutations    = 3
	unitSweepSingleStride = 8
)

func sweepDiff(base, now map[string]string) string {
	var sb strings.Builder
	for name, body := range now {
		if body != base[name] {
			fmt.Fprintf(&sb, "--- %s (changed)\n%s\n", name, body)
		}
	}
	return sb.String()
}

package borrow

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	devfixture "kigumi/internal/devkit/fixture"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

var corpusSection = regexp.MustCompile(`(?m)^-- (.+) --$`)

// TestWitnessSetsKeepNoLess: wherever a single-file run or planned sem fixture reads a generic body's own witness through
// witnessSet, the narrowed answer never keeps what no implementation of the requirement keeps. Narrowing below the whole
// requirement is the point (std/cli's Parser.flag accepts a keeping FlagValue elsewhere), so only the sound direction is pinned.
func TestWitnessSetsKeepNoLess(t *testing.T) {
	if raceDetector {
		t.Skip("the corpus pin is checked in the plain build; it keeps no results but is the heaviest borrow test")
	}
	var mu sync.Mutex
	// current is the fixture being checked: holding the sem.Result instead would keep every program's trees and types
	// alive, which is more memory than the runner has.
	current := ""
	hits, narrowed, programs := 0, 0, map[string]bool{}
	var flips []string
	witnessProbe = func(c *borrowCheck, narrow, wide []boundFn) {
		if narrow == nil {
			return
		}
		n, w := c.boundKeepAny(narrow), c.boundKeepAny(wide)
		mu.Lock()
		defer mu.Unlock()
		hits++
		programs[current] = true
		if n && !w {
			flips = append(flips, current+" fn="+c.f.Name)
		}
		if n != w {
			narrowed++
		}
	}
	std := semfixture.LoadStd(t, "../../../../../std")
	run, _ := filepath.Glob("../../../../../testdata/run/*.txtar")
	planned, _ := filepath.Glob("../../../../../testdata/sem/*.txtar")
	if len(run) == 0 || len(planned) == 0 {
		t.Fatal("no run or sem fixtures")
	}
	t.Run("corpus", func(t *testing.T) {
		for _, path := range slices.Concat(run, planned) {
			if !devfixture.Shard(filepath.Base(path)) {
				continue
			}
			data, _ := os.ReadFile(path)
			src, ok := corpusSource(string(data), strings.Contains(path, "/sem/"))
			if !ok {
				continue
			}
			// std's trees are shared by every check, so the subtests run one at a time.
			t.Run(filepath.Base(path), func(t *testing.T) {
				mu.Lock()
				current = filepath.Base(path)
				mu.Unlock()
				main := &sem.Package{Path: "main"}
				main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
				main.Files = []*syntax.Tree{main.Entry}
				res := sem.CheckWith(&sem.Module{Packages: append(slices.Clone(std), main)}, schema.GenerateDerive)
				if res.HasErrors() {
					return
				}
				buildTypedFixture(res)
			})
		}
	})
	witnessProbe = nil
	t.Logf("%d witness sets read in %d programs, %d narrower than every implementation", hits, len(programs), narrowed)
	if hits == 0 {
		t.Errorf("no fixture reads a witness set; the corpus no longer covers the narrowing")
	}
	if len(flips) > 0 {
		t.Errorf("these keep a borrow no implementation of the requirement keeps: %v", flips)
	}
}

// corpusSource is a fixture's only source file; a sem fixture counts only when its MIR is planned.
func corpusSource(data string, semFixture bool) (string, bool) {
	marks := corpusSection.FindAllStringSubmatchIndex(data, -1)
	src := ""
	for i, m := range marks {
		name := data[m[2]:m[3]]
		end := len(data)
		if i+1 < len(marks) {
			end = marks[i+1][0]
		}
		body := data[m[1]+1 : end]
		switch name {
		case "src":
			src = body
		case "module":
			if !strings.Contains(body, "mir: planned") {
				return "", false
			}
		case "stdout", "stderr", "exit", "engines", "expect", "secret-san":
		default:
			return "", false
		}
	}
	if semFixture && !strings.Contains(data, "mir: planned") {
		return "", false
	}
	return src, src != ""
}

func (c *borrowCheck) boundKeepAny(fns []boundFn) bool {
	for _, f := range fns {
		s := c.calleeRefKeeps(f.ent, f.sub)
		if c.p.R.Entity(f.ent).Kind == sem.EntFn {
			s = s.join(c.declaredRefKeeps(f.ent))
		}
		if s.keepsAny() {
			return true
		}
	}
	return false
}

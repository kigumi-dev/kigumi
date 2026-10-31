package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/driver"
)

// TestLazyStdCheckMatchesEager checks every corpus program with std bodies on demand, as units on a cold cache, and with every body, and wants the same diagnostics from check and from the editor.
func TestLazyStdCheckMatchesEager(t *testing.T) {
	for _, corpus := range []string{"sem", "mir", "run"} {
		t.Run(corpus, func(t *testing.T) {
			paths, _ := filepath.Glob(filepath.Join("../../../testdata", corpus, "*.txtar"))
			var lazy, retries, skipped, editorLazy atomic.Int64
			t.Cleanup(func() {
				t.Logf("%s: lazy=%d retries=%d skipped bodies=%d editor lazy=%d", corpus, lazy.Load(), retries.Load(), skipped.Load(), editorLazy.Load())
				if lazy.Load() == 0 {
					t.Error("no program was checked lazily")
				}
			})
			count := func(st driver.UnitStats) {
				lazy.Add(int64(st.Lazy))
				retries.Add(int64(st.LazyRetries))
				skipped.Add(int64(st.SkippedBodies))
			}
			for _, path := range paths {
				fx, ok := readSweepFixture(path)
				if !ok {
					continue
				}
				t.Run(fx.name, func(t *testing.T) {
					t.Parallel()
					root := t.TempDir()
					writeSweep(t, root, fx.files)
					outcome := func(res *sem.Result, err error) string {
						if err != nil {
							return "fault: " + err.Error()
						}
						return fmt.Sprintf("errors=%v\n%s", res.HasErrors(), res.Render())
					}
					check := func(lazyStd bool) (string, string) {
						opts := driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Test: fx.test, Entry: fx.entry, UnitCheck: lazyStd, LazyStd: lazyStd}
						editor := ""
						if em, err := driver.LoadModule(root, opts); err == nil {
							editor = outcome(driver.CheckForEditor(em))
							editorLazy.Add(int64(em.UnitStats().Lazy))
							retries.Add(int64(em.UnitStats().LazyRetries))
						}
						m, err := driver.LoadModule(root, opts)
						if err != nil {
							return "load: " + err.Error(), editor
						}
						out := outcome(driver.Check(m))
						count(m.UnitStats())
						return out, editor
					}
					got, gotEditor := check(true)
					want, wantEditor := check(false)
					if got != want || gotEditor != wantEditor {
						t.Fatalf("lazy std check differs\n--- lazy\n%s%s--- eager\n%s%s", got, gotEditor, want, wantEditor)
					}
					um := sweepLoad(t, fx, root, t.TempDir())
					units := outcome(driver.CheckUnits(um))
					count(um.UnitStats())
					if units != want {
						t.Fatalf("unit check differs from the whole check\n--- units\n%s--- whole\n%s", units, want)
					}
				})
			}
		})
	}
}

// TestLazyStdBuildMatchesEager compares what run and build produce with std bodies on demand and with all of them: the MIR, the LLVM IR and the VM's output.
func TestLazyStdBuildMatchesEager(t *testing.T) {
	manifest := loadManifest(t)
	for _, corpus := range []string{"mir", "run"} {
		t.Run(corpus, func(t *testing.T) {
			paths, _ := filepath.Glob(filepath.Join("../../../testdata", corpus, "*.txtar"))
			var lazy, retries atomic.Int64
			t.Cleanup(func() { t.Logf("%s: lazy=%d retries=%d", corpus, lazy.Load(), retries.Load()) })
			for _, path := range paths {
				fx := manifest.ByID(fixtureID(path))
				t.Run(filepath.Base(path), func(t *testing.T) {
					if !countsOpenFds(t, path) {
						t.Parallel()
					}
					root, _, _, _ := loadRunFixture(t, path)
					entry := "main/main.kg"
					if _, err := os.Stat(filepath.Join(root, entry)); err != nil {
						entry = ""
					}
					outputs := func(lazyStd bool) string {
						var sb strings.Builder
						load := func() *driver.Module {
							m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: entry, LazyStd: lazyStd})
							if err != nil {
								t.Fatal(err)
							}
							return m
						}
						m := load()
						a, err := driver.Analyze(m)
						if lazyStd {
							lazy.Add(int64(m.UnitStats().Lazy))
							retries.Add(int64(m.UnitStats().LazyRetries))
						}
						if err != nil {
							return "fault: " + err.Error()
						}
						sb.WriteString(a.Res.Render())
						if a.Prog == nil {
							return sb.String()
						}
						sb.WriteString(a.Prog.Dump())
						var errOut bytes.Buffer
						ir, ok, err := driver.Compile(load(), &errOut)
						fmt.Fprintf(&sb, "--- ir %v %v\n%s%s", ok, err, errOut.String(), ir)
						if corpus == "run" && fx != nil && fx.Runs("vm") {
							var out, errOut bytes.Buffer
							code, err := driver.RunWith(load(), driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg", "one", "two"})
							fmt.Fprintf(&sb, "--- vm %d %v\n%s--- stderr\n%s", code, err, out.String(), errOut.String())
						}
						return sb.String()
					}
					got, want := outputs(true), outputs(false)
					if got != want {
						t.Fatalf("lazy std output differs: %s", firstDiff(got, want))
					}
				})
			}
		})
	}
}

// TestLazyStdExamplesMatchEager compares the examples' diagnostics, MIR and LLVM IR with std bodies on demand and with all of them.
func TestLazyStdExamplesMatchEager(t *testing.T) {
	mods, _ := filepath.Glob("../../../examples/*/mod.kg")
	for _, mod := range mods {
		dir := filepath.Dir(mod)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			copyTree(t, dir, root)
			entry := ""
			if _, err := os.Stat(filepath.Join(root, "main.kg")); err == nil {
				entry = "main.kg"
			}
			outputs := func(lazyStd bool) (string, int) {
				load := func() *driver.Module {
					m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: entry, LazyStd: lazyStd})
					if err != nil {
						t.Fatal(err)
					}
					return m
				}
				m := load()
				a, err := driver.Analyze(m)
				if err != nil {
					return "fault: " + err.Error(), m.UnitStats().Lazy
				}
				out := a.Res.Render()
				if a.Prog != nil {
					out += a.Prog.Dump()
					var errOut bytes.Buffer
					ir, ok, err := driver.Compile(load(), &errOut)
					out += fmt.Sprintf("--- ir %v %v\n%s%s", ok, err, errOut.String(), ir)
				}
				return out, m.UnitStats().Lazy
			}
			if tests, _ := filepath.Glob(filepath.Join(root, "*", "*_test.kg")); len(tests) > 0 {
				run := func(lazyStd bool) string {
					m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: entry, Test: true, LazyStd: lazyStd})
					if err != nil {
						t.Fatal(err)
					}
					var out, errOut bytes.Buffer
					code, err := driver.RunTests(m, &out, &errOut)
					return fmt.Sprintf("%d %v\n%s--- stderr\n%s", code, err, out.String(), errOut.String())
				}
				if got, want := run(true), run(false); got != want {
					t.Fatalf("lazy std tests differ: %s", firstDiff(got, want))
				}
			}
			got, lazy := outputs(true)
			want, _ := outputs(false)
			if got != want {
				t.Fatalf("lazy std output differs: %s", firstDiff(got, want))
			}
			if lazy != 1 {
				t.Fatalf("the example was not analyzed lazily")
			}
		})
	}
}

func firstDiff(a, b string) string {
	al, bl := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return fmt.Sprintf("line %d\n lazy: %s\neager: %s", i+1, al[i], bl[i])
		}
	}
	return fmt.Sprintf("lengths %d and %d lines", len(al), len(bl))
}

func TestNoopAnalyzeWritesNothing(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	analyze := func() *driver.Module {
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: cache, LazyStd: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := driver.Analyze(m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	first := analyze()
	if cs := first.PackageCacheStats(); cs.ArtifactWrites == 0 || first.UnitStats().Lazy != 1 {
		t.Fatalf("the first build records the artifacts with std bodies on demand: %+v %+v", cs, first.UnitStats())
	}
	second := analyze()
	if cs := second.PackageCacheStats(); cs.ArtifactWrites != 0 || cs.RecordWrites != 0 {
		t.Fatalf("an unchanged module writes no artifact: %+v", cs)
	}
}

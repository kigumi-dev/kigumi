package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/driver"
)

const synthUnit = `
pub type Point%[1]d = {
    pub x Int
    pub y Int
}

pub fn Point%[1]d.len(self) -> Int {
    self.x * self.x + self.y * self.y
}

fn classify%[1]d(n: Int) -> String {
    match n {
        0 -> "zero"
        _ -> if n < 0 { "negative" } else { "positive" }
    }
}

fn sum%[1]d(xs: Array[Int]) -> Int {
    let mut total = 0
    for x in xs {
        total = total + x
    }
    total
}

pub fn makeAdder%[1]d(n: Int) -> fn(Int) -> Int {
    (m: Int) => m + n
}
`

func synthPackage(k, units, bump int) string {
	var sb strings.Builder
	if k > 0 {
		fmt.Fprintf(&sb, "import prev from synth/p%d\n\n", k-1)
	}
	for i := 0; i < units; i++ {
		fmt.Fprintf(&sb, synthUnit, i)
	}
	if k > 0 {
		fmt.Fprintf(&sb, "\npub fn total() -> Int {\n    prev.total() + %d\n}\n", 1+bump)
	} else {
		fmt.Fprintf(&sb, "\npub fn total() -> Int {\n    %d\n}\n", 1+bump)
	}
	return sb.String()
}

func synthFiles(packages, units int) map[string]string {
	files := map[string]string{
		"mod.kg":          "Module {\n    name: \"synth\"\n    kigumi: \"0.1\"\n}\n",
		"cmd/app/main.kg": fmt.Sprintf("import top from synth/p%d\n\nprint \"${top.total()}\"\n", packages-1),
	}
	for k := 0; k < packages; k++ {
		files[fmt.Sprintf("p%d/p%d.kg", k, k)] = synthPackage(k, units, 0)
	}
	return files
}

// BenchmarkPrivateEdit times a check after a private body edit in a ten package chain, whole module against units.
func BenchmarkPrivateEdit(b *testing.B) {
	for _, size := range []int{6, 40} {
		for _, edit := range []int{0, 5, 9} {
			for _, units := range []bool{false, true} {
				b.Run(fmt.Sprintf("decls=%d/edit=p%d/units=%v", size, edit, units), func(b *testing.B) {
					var load, check time.Duration
					for i := 0; i < b.N; i++ {
						b.StopTimer()
						root, cache := b.TempDir(), b.TempDir()
						writeBench(b, root, synthFiles(10, size))
						timedCheck(b, root, cache, units)
						writeBench(b, root, map[string]string{fmt.Sprintf("p%d/p%d.kg", edit, edit): synthPackage(edit, size, 1)})
						b.StartTimer()
						l, c := timedCheck(b, root, cache, units)
						load, check = load+l, check+c
					}
					b.ReportMetric(float64(load.Microseconds())/1000/float64(b.N), "load-ms")
					b.ReportMetric(float64(check.Microseconds())/1000/float64(b.N), "check-ms")
				})
			}
		}
	}
}

func writeBench(tb testing.TB, root string, files map[string]string) {
	tb.Helper()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			os.Chtimes(p, old, old)
		}
		return nil
	})
}

func timedCheck(b *testing.B, root, cache string, units bool) (load, check time.Duration) {
	b.Helper()
	std, _ := filepath.Abs("../../std")
	start := time.Now()
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std, Entry: "cmd/app/main.kg", PackageCache: cache, DeferParse: true, UnitCheck: units})
	if err != nil {
		b.Fatal(err)
	}
	loaded := time.Now()
	res, err := driver.CheckUnits(m)
	if err != nil || res.HasErrors() {
		b.Fatalf("check: %v\n%s", err, res.Render())
	}
	return loaded.Sub(start), time.Since(loaded)
}

package driver_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"kigumi/internal/driver"
)

func cpuNanos() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Utime.Nano() + ru.Stime.Nano()
}

func loadExamples(b *testing.B, only string) []*driver.Module {
	b.Helper()
	dirs, err := os.ReadDir("../../examples")
	if err != nil {
		b.Fatal(err)
	}
	var mods []*driver.Module
	for _, d := range dirs {
		if !d.IsDir() || only != "" && d.Name() != only {
			continue
		}
		m, err := driver.LoadModule(filepath.Join("../../examples", d.Name()), driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std"})
		if err != nil {
			continue
		}
		mods = append(mods, m)
	}
	return mods
}

func benchCheckCPU(b *testing.B, mods []*driver.Module) {
	start := cpuNanos()
	for range b.N {
		for _, m := range mods {
			if _, err := driver.CheckOnly(m); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportMetric(float64(cpuNanos()-start)/float64(b.N), "cpu-ns/op")
}

// BenchmarkCheckStdCPU checks std plus the smallest example.
func BenchmarkCheckStdCPU(b *testing.B) { benchCheckCPU(b, loadExamples(b, "hello")) }

// BenchmarkCheckExamplesCPU checks every example module over std.
func BenchmarkCheckExamplesCPU(b *testing.B) { benchCheckCPU(b, loadExamples(b, "")) }

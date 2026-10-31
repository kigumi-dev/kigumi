package driver_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// BenchmarkColdCheck times the first check of the ten package chain on an empty cache, whole module against units, and a first analysis as run and build do it.
func BenchmarkColdCheck(b *testing.B) {
	for _, size := range []int{6, 40} {
		for _, mode := range []string{"whole", "units", "analyze"} {
			b.Run(fmt.Sprintf("decls=%d/%s", size, mode), func(b *testing.B) {
				std, _ := filepath.Abs("../../../std")
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					root, cache := b.TempDir(), b.TempDir()
					writeBench(b, root, synthFiles(10, size))
					b.StartTimer()
					opts := driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "cmd/app/main.kg"}
					var err error
					switch mode {
					case "whole":
						var m *driver.Module
						if m, err = driver.LoadModule(root, opts); err == nil {
							_, err = driver.Check(m)
						}
					case "units":
						opts.PackageCache, opts.DeferParse, opts.UnitCheck = cache, true, true
						var m *driver.Module
						if m, err = driver.LoadModule(root, opts); err == nil {
							_, err = driver.CheckUnits(m)
						}
					case "analyze":
						opts.LazyStd = true
						var m *driver.Module
						if m, err = driver.LoadModule(root, opts); err == nil {
							_, err = driver.Analyze(m)
						}
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

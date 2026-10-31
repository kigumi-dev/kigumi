package driver_test

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// pinnedSizes are stripped x86_64-linux-musl executables in bytes, built with zig 0.17.0; repin when a change is meant to grow one.
var pinnedSizes = []struct {
	example string
	sizeMin bool
	bytes   int64
}{
	{"hello", false, 46280},
	{"word_count", false, 100264},
	{"json", false, 101784},
	{"hello", true, 25024},
	{"json", true, 65456},
}

// sizeMarginPercent is how much a pinned executable may grow before TestExecutableSizeRegression fails.
const sizeMarginPercent = 3

func TestExecutableSizeRegression(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil || driver.ZigVersionOf(cc) != "0.17.0" {
		t.Skip("the pinned sizes are zig 0.17.0's")
	}
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-linux-musl", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pinnedSizes {
		name := p.example
		if p.sizeMin {
			name += "/size-min"
		}
		t.Run(name, func(t *testing.T) {
			root, _ := filepath.Abs(filepath.Join("../../../examples", p.example))
			opts := driver.BuildOptions{Target: target, SizeMin: p.sizeMin}
			if !p.sizeMin {
				opts.LinkFlags = []string{"-s"}
			}
			info, err := os.Stat(buildRootWith(t, root, opts))
			if err != nil {
				t.Fatal(err)
			}
			got, limit := info.Size(), p.bytes+p.bytes*sizeMarginPercent/100
			t.Logf("%s: %d bytes (pinned %d)", name, got, p.bytes)
			if got > limit {
				t.Errorf("%s grew to %d bytes, over %d pinned + %d%%", name, got, p.bytes, sizeMarginPercent)
			}
		})
	}
}

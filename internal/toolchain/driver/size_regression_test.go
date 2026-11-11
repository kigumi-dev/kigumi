package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/native/link"
)

// The size baseline is pinned for the toolchain it was measured with: a
// different zig lays out code differently, so comparing across versions would
// report noise. Repin from this test's log when a change is meant to grow an
// executable.
//
// The axes are the restructure doc's (llgen_runtime_restructure_2026-11-04.md
// Phase 0.4): musl static in every pin, section GC always on for hosted ELF
// (sectionGC in build_flags.go), and the two codegen modes -O1 and the
// --size-min "-Os -flto, no unwind tables, stripped". The doc's fourth program,
// exp's tui, lives in the exp repository and is not pinned here.

// pinnedSizes are stripped x86_64-linux-musl executables in bytes, built from
// examples/ with zig 0.17.0; repin when a change is meant to grow one.
var pinnedSizes = []struct {
	example string
	sizeMin bool
	bytes   int64
}{
	{"hello", false, 45080},
	{"word_count", false, 97848},
	{"json", false, 100088},
	{"hello", true, 24640},
	{"json", true, 64768},
}

// pinnedFixtureSizes pin the same two modes for the trim and sha256 programs
// the restructure doc sizes; they only exist as run fixtures.
var pinnedFixtureSizes = []struct {
	fixture string
	sizeMin bool
	bytes   int64
}{
	{"text_trim_edge_longer.txtar", false, 55112},
	{"text_trim_edge_longer.txtar", true, 29056},
	{"crypto_sha256.txtar", false, 75608},
	{"crypto_sha256.txtar", true, 45696},
}

// sizeMarginPercent is how much a pinned executable may grow before TestExecutableSizeRegression fails.
const sizeMarginPercent = 3

func TestExecutableSizeRegression(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil || link.ZigVersionOf(cc) != "0.17.0" {
		t.Skip("the pinned sizes are zig 0.17.0's")
	}
	target, err := buildconfig.ParseTarget(testCompiler(), "x86_64-linux-musl", "")
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, name string, exe string, pinned int64) {
		t.Helper()
		info, err := os.Stat(exe)
		if err != nil {
			t.Fatal(err)
		}
		got, limit := info.Size(), pinned+pinned*sizeMarginPercent/100
		t.Logf("%s: %d bytes (pinned %d)", name, got, pinned)
		if got > limit {
			t.Errorf("%s grew to %d bytes, over %d pinned + %d%%", name, got, pinned, sizeMarginPercent)
		}
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
			check(t, name, buildRootWith(t, root, opts), p.bytes)
		})
	}
	for _, p := range pinnedFixtureSizes {
		name := p.fixture
		if p.sizeMin {
			name += "/size-min"
		}
		t.Run(name, func(t *testing.T) {
			root, _, _, _ := loadRunFixture(t, fixturePath(t, "run/"+strings.TrimSuffix(p.fixture, ".txtar")))
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			opts := driver.BuildOptions{Target: target, SizeMin: p.sizeMin}
			if !p.sizeMin {
				opts.LinkFlags = []string{"-s"}
			}
			exe := filepath.Join(root, "prog")
			var buildErr bytes.Buffer
			if ok, err := driver.BuildWith(m, exe, &buildErr, opts); err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			check(t, name, exe, p.bytes)
		})
	}
}

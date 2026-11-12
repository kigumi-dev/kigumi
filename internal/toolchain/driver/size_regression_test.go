package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
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

// pinnedSizes pin the stripped x86_64-linux-musl executables built from run
// fixtures with zig 0.17.0; repin when a change is meant to grow one.
var pinnedSizes = []struct {
	fixture string
	sizeMin bool
	bytes   int64
}{
	{"hello", false, 57304},
	{"json_derive", false, 236008},
	{"hello", true, 32184},
	{"json_derive", true, 187608},
}

// pinnedFixtureSizes pin the same two modes for the trim and sha256 programs
// the restructure doc sizes.
var pinnedFixtureSizes = []struct {
	fixture string
	sizeMin bool
	bytes   int64
}{
	{"text_trim_edge_longer", false, 56904},
	{"text_trim_edge_longer", true, 33864},
	{"crypto_sha256", false, 77416},
	{"crypto_sha256", true, 50504},
}

// sizeMarginPercent is how much a pinned executable may grow before TestExecutableSizeRegression fails.
const sizeMarginPercent = 3

func TestExecutableSizeRegression(t *testing.T) {
	if os.Getenv("KIGUMI_CFLAGS") != "" {
		t.Skip("the pinned sizes are the default C flags")
	}
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
		name := p.fixture
		if p.sizeMin {
			name += "/size-min"
		}
		t.Run(name, func(t *testing.T) {
			root, _, _, _ := loadRunFixture(t, fixturePath(t, "run/"+p.fixture))
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
			if err != nil {
				t.Fatal(err)
			}
			opts := driver.BuildOptions{Target: target, SizeMin: p.sizeMin}
			if !p.sizeMin {
				opts.LinkFlags = []string{"-s"}
			}
			exe := filepath.Join(t.TempDir(), "prog")
			var buildErr bytes.Buffer
			if ok, err := driver.BuildWith(m, exe, &buildErr, opts); err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			check(t, name, exe, p.bytes)
		})
	}
	for _, p := range pinnedFixtureSizes {
		name := p.fixture
		if p.sizeMin {
			name += "/size-min"
		}
		t.Run(name, func(t *testing.T) {
			root, _, _, _ := loadRunFixture(t, fixturePath(t, "run/"+p.fixture))
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

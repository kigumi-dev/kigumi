package cheader

import (
	"strings"
	"testing"
)

// The same header translated for glibc and for musl must land in separate cache entries with each library's values.
func TestImportCacheKeysOnTarget(t *testing.T) {
	zig := zigOrSkip(t)
	header := writeHeader(t, "#include <stdio.h>\n")
	cacheDir := t.TempDir()
	opts := Options{ZigPath: zig, HeaderPath: header, ABI: ABIFor("linux", "amd64"), CacheDir: cacheDir}
	opts.Target = "x86_64-linux-gnu"
	gnu, err := Import(opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.Target = "x86_64-linux-musl"
	musl, err := Import(opts)
	if err != nil {
		t.Fatal(err)
	}
	if musl.FromCache || musl.CachePath == gnu.CachePath {
		t.Fatalf("the musl import reused the glibc entry %s", gnu.CachePath)
	}
	if musl.Manifest.Target != "x86_64-linux-musl" || gnu.Manifest.Target != "x86_64-linux-gnu" {
		t.Errorf("manifest targets %q and %q", gnu.Manifest.Target, musl.Manifest.Target)
	}
	if !strings.Contains(gnu.Source, "pub const BUFSIZ: i32 = 8192") || !strings.Contains(musl.Source, "pub const BUFSIZ: i32 = 1024") {
		t.Error("BUFSIZ does not follow the target's C library")
	}
}

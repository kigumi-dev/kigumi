package driver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/artifact"
)

// TestActionCacheRoundTrip stores a step's outputs and restores them,
// including the executable mode, without the tool running again.
func TestActionCacheRoundTrip(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	cache := newActionCache(filepath.Join(outDir, actionCacheDirName))
	bin := filepath.Join(outDir, "tool.bin")
	obj := filepath.Join(outDir, "hello.o")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(obj, []byte("object bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := cache.store(key, []string{bin, obj}, []string{"tool.bin", "hello.o"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(obj, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !cache.lookup(key, []string{bin, obj}) {
		t.Fatal("a recorded action must be a hit")
	}
	got, err := os.ReadFile(bin)
	if err != nil || string(got) != "#!/bin/sh\n" {
		t.Fatalf("restored tool.bin = %q, %v", got, err)
	}
	if fi, err := os.Stat(bin); err != nil || fi.Mode().Perm() != 0o755 {
		t.Fatalf("restored tool.bin mode = %v, %v", fi.Mode().Perm(), err)
	}
	if got, err := os.ReadFile(obj); err != nil || string(got) != "object bytes" {
		t.Fatalf("restored hello.o = %q, %v", got, err)
	}
}

// TestActionCacheRejectsUnverifiedEntries pins the no-stale-reuse rule: a
// missing, tampered, truncated, retargeted or differently shaped entry is a
// miss, and a miss never overwrites the output with bytes it could not verify.
func TestActionCacheRejectsUnverifiedEntries(t *testing.T) {
	t.Parallel()
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cases := []struct {
		name   string
		broken func(t *testing.T, cache *actionCache, out string)
		ask    func(out string) []string
	}{
		{
			name: "missing entry",
			broken: func(t *testing.T, cache *actionCache, out string) {
				if err := os.RemoveAll(cache.entryDir(key)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "tampered blob",
			broken: func(t *testing.T, cache *actionCache, out string) {
				if err := os.WriteFile(cache.blobPath(key, 0), []byte("evil"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "truncated blob",
			broken: func(t *testing.T, cache *actionCache, out string) {
				if err := os.WriteFile(cache.blobPath(key, 0), []byte("buil"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "foreign key",
			broken: func(t *testing.T, cache *actionCache, out string) {
				b, err := os.ReadFile(cache.manifestPath(key))
				if err != nil {
					t.Fatal(err)
				}
				var e actionEntry
				if err := json.Unmarshal(b, &e); err != nil {
					t.Fatal(err)
				}
				e.Key = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
				b, err = json.Marshal(e)
				if err != nil {
					t.Fatal(err)
				}
				if err := artifact.WriteOutput(cache.manifestPath(key), b, 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "output count changed",
			broken: func(t *testing.T, cache *actionCache, out string) {
			},
			ask: func(out string) []string { return []string{out, out + ".second"} },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			outDir := t.TempDir()
			cache := newActionCache(filepath.Join(outDir, actionCacheDirName))
			out := filepath.Join(outDir, "out.txt")
			if err := os.WriteFile(out, []byte("built"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := cache.store(key, []string{out}, []string{"out.txt"}); err != nil {
				t.Fatal(err)
			}
			c.broken(t, cache, out)
			if err := os.WriteFile(out, []byte("current"), 0o644); err != nil {
				t.Fatal(err)
			}
			ask := []string{out}
			if c.ask != nil {
				ask = c.ask(out)
			}
			if cache.lookup(key, ask) {
				t.Fatal("an entry that cannot be verified must miss")
			}
			got, err := os.ReadFile(out)
			if err != nil || string(got) != "current" {
				t.Fatalf("a miss must leave the output alone, got %q, %v", got, err)
			}
		})
	}
}

// TestActionCacheRefusesSymlinkOutput checks that a planted link at an output
// path is refused instead of followed, so a cache hit cannot publish outside
// the build directory.
func TestActionCacheRefusesSymlinkOutput(t *testing.T) {
	t.Parallel()
	outDir, outside := t.TempDir(), t.TempDir()
	cache := newActionCache(filepath.Join(outDir, actionCacheDirName))
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	built := filepath.Join(outDir, "built.txt")
	if err := os.WriteFile(built, []byte("built"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := cache.store(key, []string{built}, []string{"built.txt"}); err != nil {
		t.Fatal(err)
	}
	os.Remove(built)
	victim := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, built); err != nil {
		t.Fatal(err)
	}
	if cache.lookup(key, []string{built}) {
		t.Fatal("a symlinked output must be a miss, not a write through the link")
	}
	if got, err := os.ReadFile(victim); err != nil || string(got) != "keep" {
		t.Fatalf("the file behind the link changed: %q, %v", got, err)
	}
}

// TestActionCacheDigestMemo covers the tool digest memo: an unchanged file is
// digested once, and a rewritten one is digested again.
func TestActionCacheDigestMemo(t *testing.T) {
	t.Parallel()
	cache := newActionCache(t.TempDir())
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("one"), 0o755); err != nil {
		t.Fatal(err)
	}
	first, ok := cache.digestFile(path)
	if !ok {
		t.Fatal("a regular file must digest")
	}
	if again, ok := cache.digestFile(path); !ok || again != first {
		t.Fatalf("an unchanged file re-digested: %s, %s (%v)", first, again, ok)
	}
	if err := os.WriteFile(path, []byte("one and more"), 0o755); err != nil {
		t.Fatal(err)
	}
	changed, ok := cache.digestFile(path)
	if !ok || changed == first {
		t.Fatalf("a rewritten file kept its digest: %s, %s (%v)", first, changed, ok)
	}
	if _, ok := cache.digestFile(t.TempDir()); ok {
		t.Error("a directory must not digest as an input")
	}
}

package cheader

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestPublishEntryNeverExposesPartialDirectory races duplicate publishes
// of the same content-addressed dir against readers, and checks a reader
// never sees manifest.json without a matching gen.kg.
func TestPublishEntryNeverExposesPartialDirectory(t *testing.T) {
	cacheDir := t.TempDir()
	dir := filepath.Join(cacheDir, "entry")
	mf := Manifest{FormatVersion: formatVersion, Skipped: []string{"s"}}
	const source = "pub const x: i32 = 1\n"
	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				if err := publishEntry(cacheDir, dir, source, mf); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			got, ok := readManifest(dir)
			if !ok {
				continue
			}
			src, err := os.ReadFile(filepath.Join(dir, "gen.kg"))
			if err != nil {
				t.Errorf("manifest.json published without gen.kg: %v", err)
				return
			}
			if string(src) != source || len(got.Skipped) != 1 || got.Skipped[0] != "s" {
				t.Errorf("torn publish: skipped %v, source %q", got.Skipped, src)
				return
			}
		}
	}()
	wg.Wait()
}

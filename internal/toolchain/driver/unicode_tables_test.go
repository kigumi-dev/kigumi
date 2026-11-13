package driver_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnicodeTablesArePinned guards the generated std/text tables. Their
// generator lives in exp/unicode/tools, so this repository cannot regenerate
// them; pinning the UCD version and content catches a hand edit or a partial
// regeneration. Update both pins in the same change that ships new UCD data.
func TestUnicodeTablesArePinned(t *testing.T) {
	const ucd = "UCD 18.0.0"
	want := map[string]string{
		"unicode_tables.kg": "c7ebbc9d164a8603b81925f662b2e8ef011a7b701fcad2f804411efcd4f806c0",
		"unicode_lookup.kg": "2d1d462b166bf4b8671e7aa49428f6386d046a2b541d672ec6265d7915868346",
	}
	for name, hash := range want {
		data, err := os.ReadFile(filepath.Join("../../../std/text", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data[:min(len(data), 256)]), ucd) {
			t.Errorf("%s does not name %s", name, ucd)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != hash {
			t.Errorf("%s content hash = %s, want %s; regenerate the tables instead of editing them", name, got, hash)
		}
	}
}

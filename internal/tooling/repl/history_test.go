package repl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryModeAndDedup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "history")
	h, err := NewHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	h.Add("one")
	h.Add("one")
	h.Add("two")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("history mode = %o", st.Mode().Perm())
	}
	h2, err := NewHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(h2.lines) != 2 || h2.lines[0] != "one" || h2.lines[1] != "two" {
		t.Fatalf("history = %v", h2.lines)
	}
}

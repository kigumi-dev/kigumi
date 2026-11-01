package session

import (
	"os"
	"path/filepath"
	"testing"
)

// A lone script with no ancestor mod.kg/cmd roots at its own directory, not the grandparent: an adjacent
// mod.local.kg must be visible and a sibling script one level up must stay out of the load.
func TestModuleRootNoAncestorFallsBackToOwnDir(t *testing.T) {
	root := t.TempDir()
	scriptDir := filepath.Join(root, "lonescript")
	os.MkdirAll(scriptDir, 0o755)
	entry := filepath.Join(scriptDir, "main.kg")
	os.WriteFile(entry, []byte("print 1\n"), 0o644)

	if got := moduleRoot(entry); got != scriptDir {
		t.Errorf("moduleRoot(%s) = %s, want %s", entry, got, scriptDir)
	}
}

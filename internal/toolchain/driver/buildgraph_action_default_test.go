package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildToolsRerunByDefault(t *testing.T) {
	t.Parallel()
	root, out := actionRoot(t), t.TempDir()
	hidden := filepath.Join(out, "hidden.txt")
	script := "#!/bin/sh\nprintf 'run\\n' >> runs.log\ncat hidden.txt > \"$2\"\n"
	for _, body := range []string{"first\n", "changed\n"} {
		if err := os.WriteFile(hidden, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		log := actionBuildWith(t, root, out, "1", script, false)
		if strings.Contains(log, "reuses cached outputs") {
			t.Fatal(log)
		}
		got, err := os.ReadFile(filepath.Join(out, "copy.txt"))
		if err != nil || string(got) != body {
			t.Fatalf("output %q, want %q: %v", got, body, err)
		}
	}
	if got := actionRuns(t, out); got != 2 {
		t.Fatalf("tool ran %d times, want 2", got)
	}
	if _, err := os.Stat(filepath.Join(out, ".kigumi-actions")); !os.IsNotExist(err) {
		t.Fatalf("default build created an action cache: %v", err)
	}
}

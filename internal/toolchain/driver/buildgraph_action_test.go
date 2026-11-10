package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// actionRoot writes a module whose build.kg runs one Framework tool over a
// declared input, with the Framework version and the tool script under the
// test's control.
func actionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mod.kg", "Module {\n    name: \"act\"\n    kigumi: \"0.1\"\n}\n")
	write("note.txt", "first\n")
	return root
}

// actionBuild is one whole build.kg run into out, returning what the driver
// reported on stderr.
func actionBuild(t *testing.T, root, out, version, script string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "kigumi.framework.kg"), []byte(
		"Framework { name: \"t/act\", version: \""+version+"\" }\nTool { name: \"copy\", program: \"tools/copy.sh\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(root, "tools", "copy.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	program := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n" +
		"    let fw = b.framework(\"t/act\", \"" + version + "\")\n" +
		"    let src = b.file(\"note.txt\")?\n" +
		"    let o = b.runToolOutput(fw, \"copy\", Array.of(\"$in:0\", \"$out:0\"), Array.of(src), \"copy.txt\", Array.empty[build.Secret]())?\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(root, "build.kg"), []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil || !ok {
		t.Fatalf("load build.kg: ok=%v err=%v", ok, err)
	}
	g, err := p.Run(buildconfig.HostTarget(), nil, out)
	if err != nil {
		t.Fatal(err)
	}
	roots, _ := p.DescriptorRoots()
	descriptors, err := driver.LoadFrameworks(roots)
	if err != nil {
		t.Fatal(err)
	}
	rg, err := driver.ResolveGraph(testCompiler(), g, descriptors)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if err := driver.ExecuteGraph(p, rg, out, driver.ExecOptions{ToolEnv: []string{"PATH=" + os.Getenv("PATH")}}, &stderr); err != nil {
		t.Fatalf("execute: %v\n%s", err, stderr.String())
	}
	return stderr.String()
}

const actionScript = "#!/bin/sh\nprintf 'run\\n' >> runs.log\ncat \"$1\" > \"$2\"\n"

// actionRuns counts how often the tool process actually executed.
func actionRuns(t *testing.T, out string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(out, "runs.log"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "run\n")
}

// TestBuildActionReuseAndSplit covers ARCH-12 end to end: an identical
// tool/argv/env/input action is reused, while a changed input, a rewritten
// tool executable or a changed tool version each produce a new action, so no
// old output is ever reused.
func TestBuildActionReuseAndSplit(t *testing.T) {
	t.Parallel()
	root, out := actionRoot(t), t.TempDir()

	log := actionBuild(t, root, out, "1", actionScript)
	if !strings.Contains(log, "runs trusted") {
		t.Fatalf("the first action must run, log:\n%s", log)
	}
	if got, err := os.ReadFile(filepath.Join(out, "copy.txt")); err != nil || string(got) != "first\n" {
		t.Fatalf("first output = %q, %v", got, err)
	}
	if actionRuns(t, out) != 1 {
		t.Fatalf("first build ran the tool %d times", actionRuns(t, out))
	}

	// The same action: the tool must not run and the output must come back.
	if err := os.Remove(filepath.Join(out, "copy.txt")); err != nil {
		t.Fatal(err)
	}
	log = actionBuild(t, root, out, "1", actionScript)
	if !strings.Contains(log, "reuses cached outputs") {
		t.Fatalf("an identical action should be reused, log:\n%s", log)
	}
	if actionRuns(t, out) != 1 {
		t.Fatalf("an identical action ran the tool again (%d runs)", actionRuns(t, out))
	}
	if got, err := os.ReadFile(filepath.Join(out, "copy.txt")); err != nil || string(got) != "first\n" {
		t.Fatalf("restored output = %q, %v", got, err)
	}

	// A changed declared input splits the action and refreshes the output.
	if err := os.WriteFile(filepath.Join(root, "note.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	actionBuild(t, root, out, "1", actionScript)
	if got, err := os.ReadFile(filepath.Join(out, "copy.txt")); err != nil || string(got) != "second\n" {
		t.Fatalf("output after an input change = %q, %v", got, err)
	}
	if actionRuns(t, out) != 2 {
		t.Fatalf("a changed input reused the action (%d runs)", actionRuns(t, out))
	}

	// A rewritten tool executable splits the action even with identical argv,
	// env and inputs, and its own output replaces the stale one.
	actionBuild(t, root, out, "1", actionScript+"printf 'patched\\n' >> \"$2\"\n")
	if actionRuns(t, out) != 3 {
		t.Fatalf("a rewritten tool reused the action (%d runs)", actionRuns(t, out))
	}
	if got, err := os.ReadFile(filepath.Join(out, "copy.txt")); err != nil || !strings.Contains(string(got), "patched") {
		t.Fatalf("output after the tool changed = %q, %v", got, err)
	}

	// The negative test for a tool version change: the executable bytes,
	// argv, env and inputs are identical, so only the declared version can
	// tell the two tools apart.
	actionBuild(t, root, out, "2", actionScript+"printf 'patched\\n' >> \"$2\"\n")
	if actionRuns(t, out) != 4 {
		t.Fatalf("a tool version change reused the recorded action (%d runs)", actionRuns(t, out))
	}
}

package vm_test

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

// TestShellCaptureDoesNotLeakFds runs 40 external and 40 external+filter
// captures and checks the descriptor table returns to its baseline after each
// batch: every stage's pipe reader must be closed once the capture ends.
func TestShellCaptureDoesNotLeakFds(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("counts /proc/self/fd")
	}
	src := "import shell from std/shell\n" +
		"import coreutil from std/shell/coreutil\n" +
		"import fs from std/fs\n" +
		"\n" +
		"let sh = host.shell()\n" +
		"let files = host.files()\n" +
		"\n" +
		"fn nfd(files: fs.Files) -> usize {\n" +
		"    match fs.Path.fromString(\"/proc/self/fd\") {\n" +
		"        Ok(p) -> match files.list(p) {\n" +
		"            Ok(entries) -> entries.len()\n" +
		"            Err(_) -> 999\n" +
		"        }\n" +
		"        Err(_) -> 998\n" +
		"    }\n" +
		"}\n" +
		"\n" +
		"print \"before=${nfd(files)}\"\n" +
		"let mut i = 0\n" +
		"for i < 40 {\n" +
		"    match $\"printf x\" |> shell.capture(sh) { Ok(_) -> {}, Err(_) -> {} }\n" +
		"    i += 1\n" +
		"}\n" +
		"print \"after-external=${nfd(files)}\"\n" +
		"let mut j = 0\n" +
		"for j < 40 {\n" +
		"    match $\"printf x\" |> coreutil.cat |> shell.capture(sh) { Ok(_) -> {}, Err(_) -> {} }\n" +
		"    j += 1\n" +
		"}\n" +
		"print \"after-filter=${nfd(files)}\"\n"

	out, errOut, code := runVMProgram(t, src, vm.Options{})
	if code != 0 {
		t.Fatalf("exit %d\nstderr: %s", code, errOut)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("stdout = %q, want three fd counts", out)
	}
	counts := make([]int, len(lines))
	for i, line := range lines {
		_, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("line %q has no count", line)
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("count in %q: %v", line, err)
		}
		counts[i] = n
		if n > 100 {
			t.Fatalf("fd count %d in %q looks like the /proc listing failed", n, line)
		}
	}
	if counts[1] != counts[0] {
		t.Errorf("fds after 40 external captures = %d, baseline %d (%s)", counts[1], counts[0], out)
	}
	if counts[2] != counts[0] {
		t.Errorf("fds after 40 filter captures = %d, baseline %d (%s)", counts[2], counts[0], out)
	}
}

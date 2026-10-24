package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func TestBuildProgramContextKeyRejected(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"ctxbp\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte(strings.Join([]string{
		"import build from std/build",
		"import {stdout} from std/stdio",
		"",
		"fn configure(b: build.Builder) -> Void! {",
		"    _ = stdout.write(\"configuring\\n\".toBytes())?",
		"    let a = b.executable(\"app\", \"\", b.request().target())?",
		"}",
	}, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	_, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if !ok || err == nil {
		t.Fatalf("expected configure's stdout read to be rejected, got ok=%v err=%v", ok, err)
	}
	if !strings.Contains(err.Error(), "E2258") || !strings.Contains(err.Error(), "build program entry `configure` requires context key `stdout`") {
		t.Fatalf("expected E2258 naming configure and stdout:\n%v", err)
	}
}

func TestBuildProgramContextOpenedInsideIsAccepted(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"ctxbpok\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "build.kg"), []byte(strings.Join([]string{
		"import build from std/build",
		"import io from std/io",
		"import stdio from std/stdio",
		"import {stdout} from std/stdio",
		"",
		"fn configure(b: build.Builder) -> Void! {",
		"    let streams = stdio.Stdio { stdin: io.BytesReader.new(\"\".toBytes()), stdout: io.BytesWriter.new(), stderr: io.BytesWriter.new() }",
		"    context (streams) {",
		"        _ = stdout.write(\"configuring\\n\".toBytes())?",
		"    }",
		"    let a = b.executable(\"app\", \"\", b.request().target())?",
		"}",
	}, "\n")+"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"hi\"\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	_, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if !ok || err != nil {
		t.Fatalf("a build program that opens its own context must load, got ok=%v err=%v", ok, err)
	}
}

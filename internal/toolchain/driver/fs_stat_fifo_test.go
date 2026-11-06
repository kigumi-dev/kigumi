package driver_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestFilesStatFifoDoesNotBlock(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("mkfifo is Linux-only here")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	src := `import fs from std/fs

let files = host.files()
let p = fs.Path.fromString("` + fifo + `").unwrap()
let st = files.stat(p).unwrap()
print "${st.kind is fs.Kind.Other} ${files.isFile(p).unwrap()}"
`
	exe := buildProgram(t, src)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, exe).CombinedOutput()
	if err != nil {
		t.Fatalf("stat on a fifo: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "true false" {
		t.Fatalf("got %q, want %q", got, "true false")
	}
}

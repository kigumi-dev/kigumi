package repl

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"kigumi/internal/devkit/pty"
)

func TestEditorRawRestoreAndLine(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	before, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Skipf("termios: %v", err)
	}
	h, _ := NewHistory("")
	ed, err := NewEditor(slave, slave, h)
	if err != nil {
		t.Fatal(err)
	}
	if !ed.IsTerminal() {
		t.Fatal("editor did not enter raw mode")
	}
	got := make(chan string, 1)
	go func() {
		line, _ := ed.ReadLine("> ")
		got <- line
	}()
	// Drain the prompt/redraw output until it asks for input.
	buf := make([]byte, 256)
	_ = master.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		n, err := master.Read(buf)
		if err != nil {
			t.Fatalf("read prompt: %v", err)
		}
		if strings.Contains(string(buf[:n]), ">") {
			break
		}
	}
	if _, err := master.WriteString("let x\r"); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-got:
		if line != "let x" {
			t.Fatalf("line = %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ReadLine timed out")
	}
	ed.Close()
	after, err := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if after.Lflag != before.Lflag || after.Iflag != before.Iflag {
		t.Fatalf("termios not restored: before %+v after %+v", before, after)
	}
}

func TestEditorCtrlC(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("pty: %v", err)
	}
	defer master.Close()
	defer slave.Close()
	ed, err := NewEditor(slave, slave, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ed.Close()
	go func() {
		buf := make([]byte, 64)
		_, _ = master.Read(buf)
		_, _ = master.Write([]byte{3})
	}()
	line, err := ed.ReadLine("> ")
	if err != ErrInterrupted || line != "" {
		t.Fatalf("line=%q err=%v", line, err)
	}
}

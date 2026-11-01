package lsp_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/toolchain/version"
)

func TestHelp(t *testing.T) {
	code, out, errOut := clitest.Execute(t, "lsp", "--help")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	clitest.Golden(t, "help.golden", out)
}

func TestInvalidTarget(t *testing.T) {
	code, errOut := clitest.ExecuteReal(t, "lsp", "--target", "bogus-x")
	if code != 1 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	clitest.Golden(t, "invalid_target.golden", errOut)
}

// TestSession drives an initialize, shutdown and exit exchange over swapped stdin and stdout, waiting for each reply
// like a client does: the server may still be answering when its input ends, so the input cannot be written at once.
func TestSession(t *testing.T) {
	std, _ := filepath.Abs("../../../../std")
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW
	defer func() { os.Stdin, os.Stdout = stdin, stdout }()
	var replies []string
	collected := make(chan struct{})
	go func() {
		defer close(collected)
		r := bufio.NewReader(outR)
		send := func(msg string) { fmt.Fprintf(inW, "Content-Length: %d\r\n\r\n%s", len(msg), msg) }
		send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"processId":null,"rootUri":null,"capabilities":{}}}`)
		replies = append(replies, readUntil(r, `"id":1`)...)
		send(`{"jsonrpc":"2.0","method":"initialized","params":{}}`)
		send(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
		replies = append(replies, readUntil(r, `"id":2`)...)
		send(`{"jsonrpc":"2.0","method":"exit"}`)
		inW.Close()
		io.Copy(io.Discard, r)
	}()
	code, _, errOut := clitest.Execute(t, "--std", std, "lsp")
	outW.Close()
	<-collected
	os.Stdin, os.Stdout = stdin, stdout
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	out := strings.Join(replies, "\n") + "\n"
	if !strings.Contains(out, `"capabilities"`) {
		t.Fatalf("no initialize reply:\n%s", out)
	}
	clitest.Golden(t, "session.golden", messages(out, std))
}

// readUntil returns the framed messages up to and including the one containing marker, or what arrived before EOF.
func readUntil(r *bufio.Reader, marker string) []string {
	var out []string
	for {
		msg, ok := readFrame(r)
		if !ok {
			return out
		}
		out = append(out, msg)
		if strings.Contains(msg, marker) {
			return out
		}
	}
}

func readFrame(r *bufio.Reader) (string, bool) {
	n := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return "", false
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if v, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			n, _ = strconv.Atoi(v)
		}
	}
	if n < 0 {
		return "", false
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return "", false
	}
	return string(body), true
}

// messages lists the replies one per line, with the machine's std path and the release masked.
func messages(out, std string) string {
	var lines []string
	for _, body := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		body = strings.ReplaceAll(body, std, "<std>")
		body = strings.ReplaceAll(body, `"version":"`+version.Release+`"`, `"version":"<release>"`)
		lines = append(lines, body)
	}
	return strings.Join(lines, "\n") + "\n"
}

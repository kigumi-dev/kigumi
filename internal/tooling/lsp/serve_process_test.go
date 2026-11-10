package lsp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	buildconfig "kigumi/internal/toolchain/config"
)

func TestServeHelperProcess(t *testing.T) {
	if os.Getenv("KIGUMI_LSP_SERVE_HELPER") == "" {
		t.Skip("helper for the process tests")
	}
	if err := Serve("", buildconfig.HostTarget(), "", internalTestCompiler(), false); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

type serveProc struct {
	t     *testing.T
	cmd   *exec.Cmd
	stdin io.WriteCloser
	r     *bufio.Reader
	c     *rawClient
}

func startServeProc(t *testing.T) *serveProc {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestServeHelperProcess$")
	cmd.Env = append(os.Environ(), "KIGUMI_LSP_SERVE_HELPER=1")
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return &serveProc{t: t, cmd: cmd, stdin: in, r: bufio.NewReader(out)}
}

func (p *serveProc) frame(body string) {
	fmt.Fprintf(p.stdin, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func (p *serveProc) body() string {
	p.t.Helper()
	n := 0
	for {
		line, err := p.r.ReadString('\n')
		if err != nil {
			p.t.Fatalf("reading a reply: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		fmt.Sscanf(line, "Content-Length: %d", &n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(p.r, b); err != nil {
		p.t.Fatal(err)
	}
	return string(b)
}

func (p *serveProc) exitCode() int {
	p.t.Helper()
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case err := <-done:
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 0
	case <-time.After(10 * time.Second):
		p.t.Fatal("the process kept running after exit while stdin stayed open")
		return -1
	}
}

func TestServeProcessRecoversAndExitsWithTheLifecycleCode(t *testing.T) {
	for name, shutdown := range map[string]bool{"exit before shutdown": false, "exit after shutdown": true} {
		t.Run(name, func(t *testing.T) {
			p := startServeProc(t)
			fmt.Fprint(p.stdin, "Content-Length: 6\r\n\r\n{abc:}")
			if b := p.body(); !strings.Contains(b, "-32700") {
				t.Fatalf("a malformed frame must be answered with -32700, got %s", b)
			}
			p.frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":` + initParamsJSON + `}`)
			for {
				if b := p.body(); strings.Contains(b, `"id":1`) {
					break
				}
			}
			want := 1
			if shutdown {
				p.frame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
				for !strings.Contains(p.body(), `"id":2`) {
				}
				want = 0
			}
			p.frame(`{"jsonrpc":"2.0","method":"exit"}`)
			if got := p.exitCode(); got != want {
				t.Fatalf("exit code %d, want %d", got, want)
			}
		})
	}
}

func TestResyncIsSilentAfterTheFirstReplyAndRecoversOnAValidFrame(t *testing.T) {
	c, _ := serveOverPipe(t, newServer(""))
	go c.raw("Content-Type: x\r\n\r\n" + `{"a":1}` + "\r\nplain\r\n\r\n")
	errs := 0
	if m, err := c.recv(); err != nil || m.Error == nil {
		t.Fatalf("the first broken frame must be answered: %+v, %v", m, err)
	}
	errs++
	c.call(5, "initialize", initParamsJSON)
	for {
		m, err := c.recv()
		if err != nil {
			t.Fatal(err)
		}
		if string(m.ID) == "5" {
			break
		}
		if m.Error != nil {
			errs++
		}
	}
	if errs != 1 {
		t.Fatalf("one broken frame should be answered once, got %d replies", errs)
	}
}

func TestLateBodyIsHandledNotDropped(t *testing.T) {
	c, _ := serveOverPipe(t, newServer(""))
	body := `{"jsonrpc":"2.0","id":7,"method":"initialize","params":` + initParamsJSON + `}`
	c.raw(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body)))
	time.Sleep(900 * time.Millisecond)
	c.raw(body)
	for {
		m, err := c.recv()
		if err != nil {
			t.Fatal(err)
		}
		if m.Error != nil {
			t.Fatalf("a body that arrives late is still a valid frame: %+v", m.Error)
		}
		if string(m.ID) == "7" {
			return
		}
	}
}

func TestTruncatedBodyIsAnsweredAtEOF(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer ln.Close()
	accepted := make(chan net.Conn, 1)
	go func() { c, _ := ln.Accept(); accepted <- c }()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	done := make(chan error, 1)
	go func() { done <- newServer("").serve(<-accepted) }()
	c := &rawClient{t: t, c: client, r: newBufReader(client)}
	c.raw("Content-Length: 200\r\n\r\n{\"jsonrpc\":\"2.0\"}")
	client.(*net.TCPConn).CloseWrite()
	m, err := c.recv()
	if err != nil || m.Error == nil || m.Error.Code != -32700 {
		t.Fatalf("truncated frame reply: %+v, %v", m, err)
	}
	if err := waitServe(t, done); err != nil {
		t.Fatalf("EOF is a clean end: %v", err)
	}
}

func TestNullIdAndBadBatchMembersGetInvalidRequest(t *testing.T) {
	c, _ := serveOverPipe(t, newServer(""))
	c.frame(`{"jsonrpc":"2.0","id":null,"method":"initialize","params":` + initParamsJSON + `}`)
	if m, err := c.recv(); err != nil || m.Error == nil || m.Error.Code != -32600 || string(m.ID) != "null" {
		t.Fatalf("a request with id null: %+v, %v", m, err)
	}
	c.frame(`[{"jsonrpc":"2.0","id":3,"method":"initialize","params":` + initParamsJSON + `},1]`)
	for {
		body, err := c.body()
		if err != nil {
			t.Fatal(err)
		}
		if body[0] != '[' {
			continue
		}
		if strings.Contains(string(body), "-32700") || !strings.Contains(string(body), "-32600") {
			t.Fatalf("a non-object batch member must be -32600: %s", body)
		}
		return
	}
}

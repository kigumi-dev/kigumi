package lsp

import (
	"net"
	"strings"
	"testing"
	"time"
)

func serveOverPipe(t *testing.T, s *Server) (*rawClient, <-chan error) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	done := make(chan error, 1)
	go func() { done <- s.serve(a) }()
	return &rawClient{t: t, c: b, r: newBufReader(b)}, done
}

func waitServe(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return while the client kept the connection open")
		return nil
	}
}

func TestServeReturnsAfterExitWithoutWaitingForEOF(t *testing.T) {
	for name, shutdown := range map[string]bool{"before shutdown": false, "after shutdown": true} {
		t.Run(name, func(t *testing.T) {
			c, done := serveOverPipe(t, newServer(""))
			c.call(1, "initialize", initParamsJSON)
			c.errCode(1)
			if shutdown {
				c.call(2, "shutdown", "null")
				c.errCode(2)
			}
			c.notify("exit")
			err := waitServe(t, done)
			if shutdown && err != nil {
				t.Fatalf("exit after shutdown: %v", err)
			}
			if !shutdown && err != ErrExitWithoutShutdown {
				t.Fatalf("exit before shutdown: %v", err)
			}
		})
	}
}

func TestServeAnswersMalformedFrameThenKeepsServing(t *testing.T) {
	c, done := serveOverPipe(t, newServer(""))
	c.raw("Content-Length: 6\r\n\r\n{abc:}")
	if m, err := c.recv(); err != nil || m.Error == nil || m.Error.Code != -32700 {
		t.Fatalf("parse error reply: %+v, %v", m, err)
	}
	c.call(1, "initialize", initParamsJSON)
	if got := c.errCode(1); got != 0 {
		t.Fatalf("initialize after a bad frame: %d", got)
	}
	c.notify("exit")
	waitServe(t, done)
}

func TestSlowSteadyBodyIsAnsweredOnce(t *testing.T) {
	c, _ := serveOverPipe(t, newServer(""))
	body := `{"jsonrpc":"2.0","id":7,"method":"initialize","params":` + initParamsJSON + `}` + strings.Repeat(" ", 160)
	c.raw("Content-Length: " + itoa(len(body)) + "\r\n\r\n")
	for i := 0; i < len(body); i += 10 {
		c.raw(body[i:min(i+10, len(body))])
		time.Sleep(100 * time.Millisecond)
	}
	for {
		m, err := c.recv()
		if err != nil {
			t.Fatal(err)
		}
		if m.Error != nil {
			t.Fatalf("a slow but steady body got an error reply: %+v", m.Error)
		}
		if string(m.ID) == "7" {
			return
		}
	}
}

func TestBatchStillDispatches(t *testing.T) {
	c, _ := serveOverPipe(t, newServer(""))
	c.frame(`[{"jsonrpc":"2.0","id":1,"method":"initialize","params":` + initParamsJSON + `}]`)
	for {
		body, err := c.body()
		if err != nil {
			t.Fatal(err)
		}
		if body[0] == '[' {
			return
		}
		if strings.Contains(string(body), `"id":1`) {
			t.Fatalf("a batch must be answered with a batch array, got %s", body)
		}
	}
}

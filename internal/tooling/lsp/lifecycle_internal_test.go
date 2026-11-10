package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

type rawClient struct {
	t *testing.T
	c net.Conn
	r *bufio.Reader
}

func newRawClient(t *testing.T, s *Server) *rawClient {
	t.Helper()
	a, b := net.Pipe()
	s.Connect(context.Background(), newRecoveringStream(a))
	t.Cleanup(func() { a.Close(); b.Close() })
	return &rawClient{t: t, c: b, r: newBufReader(b)}
}

func (c *rawClient) raw(s string) {
	c.t.Helper()
	c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(c.c, s); err != nil {
		c.t.Fatal(err)
	}
}

func (c *rawClient) frame(body string) {
	c.raw(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

func (c *rawClient) call(id int, method, params string) {
	c.frame(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, id, method, params))
}

func (c *rawClient) notify(method string) {
	c.frame(fmt.Sprintf(`{"jsonrpc":"2.0","method":%q}`, method))
}

func (c *rawClient) body() ([]byte, error) {
	c.c.SetReadDeadline(time.Now().Add(10 * time.Second))
	n := 0
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if v, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			n, _ = strconv.Atoi(v)
		}
	}
	body := make([]byte, n)
	_, err := io.ReadFull(c.r, body)
	return body, err
}

type rawMsg struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

func (c *rawClient) recv() (msg rawMsg, err error) {
	body, err := c.body()
	if err != nil {
		return msg, err
	}
	return msg, json.Unmarshal(body, &msg)
}

func (c *rawClient) errCode(id int) int {
	c.t.Helper()
	for {
		m, err := c.recv()
		if err != nil {
			c.t.Fatalf("waiting for the reply to %d: %v", id, err)
		}
		if string(m.ID) == strconv.Itoa(id) {
			if m.Error == nil {
				return 0
			}
			return m.Error.Code
		}
	}
}

const initParamsJSON = `{"processId":null,"rootUri":null,"capabilities":{}}`

func TestLifecycleRejectsOutOfOrderMessages(t *testing.T) {
	s := newServer("")
	c := newRawClient(t, s)
	hover := `{"textDocument":{"uri":"file:///x.kg"},"position":{"line":0,"character":0}}`

	c.call(1, "textDocument/hover", hover)
	if got := c.errCode(1); got != -32002 {
		t.Fatalf("request before initialize: code %d, want -32002", got)
	}
	c.call(2, "initialize", initParamsJSON)
	if got := c.errCode(2); got != 0 {
		t.Fatalf("first initialize failed with %d", got)
	}
	c.call(3, "initialize", initParamsJSON)
	if got := c.errCode(3); got != -32600 {
		t.Fatalf("second initialize: code %d, want -32600", got)
	}
	c.call(4, "shutdown", "null")
	if got := c.errCode(4); got != 0 {
		t.Fatalf("shutdown failed with %d", got)
	}
	c.call(5, "textDocument/hover", hover)
	if got := c.errCode(5); got != -32600 {
		t.Fatalf("request after shutdown: code %d, want -32600", got)
	}
	c.notify("exit")
	select {
	case <-s.life.exited:
	case <-time.After(5 * time.Second):
		t.Fatal("exit was not observed")
	}
	if err := s.life.exitErr(); err != nil {
		t.Fatalf("exit after shutdown must be clean: %v", err)
	}
}

func TestExitBeforeShutdownIsNotClean(t *testing.T) {
	s := newServer("")
	c := newRawClient(t, s)
	c.call(1, "initialize", initParamsJSON)
	c.errCode(1)
	c.notify("exit")
	<-s.life.exited
	if err := s.life.exitErr(); err != ErrExitWithoutShutdown {
		t.Fatalf("exit before shutdown: %v", err)
	}
}

func TestMalformedFramesAreAnsweredAndSkipped(t *testing.T) {
	good := `{"jsonrpc":"2.0","id":7,"method":"initialize","params":` + initParamsJSON + `}`
	cases := map[string]string{
		"invalid json":      "Content-Length: 6\r\n\r\n{abc:}",
		"short length":      fmt.Sprintf("Content-Length: 10\r\n\r\n%s", good),
		"not an object":     "Content-Length: 2\r\n\r\n[]",
		"missing length":    "Content-Type: x\r\nbroken\r\n\r\n",
		"colon only header": "Content-Type: x\r\n\r\n",
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			c := newRawClient(t, newServer(""))
			c.raw(bad)
			if m, err := c.recv(); err != nil || m.Error == nil || (m.Error.Code != -32700 && m.Error.Code != -32600) {
				t.Fatalf("malformed frame reply: %+v, %v", m, err)
			}
			c.frame(good)
			if got := c.errCode(7); got != 0 {
				t.Fatalf("server did not keep serving: %d", got)
			}
		})
	}
}

func newBufReader(c net.Conn) *bufio.Reader { return bufio.NewReader(c) }

func itoa(n int) string { return strconv.Itoa(n) }

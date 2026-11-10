package lsp

import "testing"

// A frame whose Content-Length overstates the bytes the client sends is
// answered with -32700 without waiting for the client to hang up, and the
// connection keeps serving.
func TestOverstatedLengthIsAnsweredWithoutEOF(t *testing.T) {
	c, done := serveOverPipe(t, newServer(""))
	c.raw("Content-Length: 200\r\n\r\n" + `{"jsonrpc":"2.0","id":9,"method":"x"}`)
	if m, err := c.recv(); err != nil || m.Error == nil || m.Error.Code != -32700 {
		t.Fatalf("overstated Content-Length reply: %+v, %v", m, err)
	}
	c.call(1, "initialize", initParamsJSON)
	if got := c.errCode(1); got != 0 {
		t.Fatalf("initialize after a short body: %d", got)
	}
	c.notify("exit")
	waitServe(t, done)
}

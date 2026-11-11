package net

import (
	"bytes"
	"testing"
	"time"
)

func loopbackPort(t *testing.T, fd int) int64 {
	t.Helper()
	port := Port(fd)
	if port == 0 {
		t.Fatal("listening fd has no port")
	}
	return int64(port)
}

// TestLoopbackExchange drives the blocking path over a real IPv4 loopback
// socket, so both the Unix and Windows implementations are exercised whenever
// the package's tests run on that host.
func TestLoopbackExchange(t *testing.T) {
	l, err := Listen("127.0.0.1", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(l)
	port := loopbackPort(t, l)

	type accepted struct {
		fd  int
		err error
	}
	done := make(chan accepted, 1)
	go func() {
		fd, err := Accept(l)
		done <- accepted{fd: fd, err: err}
	}()

	c, err := Dial("127.0.0.1", port)
	if err != nil {
		t.Fatalf("dial 127.0.0.1:%d: %v", port, err)
	}
	defer Close(c)
	var s int
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("accept: %v", got.err)
		}
		s = got.fd
	case <-time.After(5 * time.Second):
		t.Fatal("accept timed out")
	}
	defer Close(s)

	if n, err := Write(c, []byte("ping")); err != nil || n != 4 {
		t.Fatalf("write = %d, %v", n, err)
	}
	buf := make([]byte, 4)
	for read := 0; read < len(buf); {
		n, err := Read(s, buf[read:])
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		read += n
	}
	if !bytes.Equal(buf, []byte("ping")) {
		t.Fatalf("read %q", buf)
	}
}

// TestTryOperations checks the non-blocking path: an idle listener and socket
// report "not ready" instead of blocking, and data becomes visible to TryRead
// after TryWrite.
func TestTryOperations(t *testing.T) {
	l, err := Listen("127.0.0.1", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(l)
	port := loopbackPort(t, l)

	if _, ok, err := TryAccept(l); err != nil || ok {
		t.Fatalf("TryAccept on an idle listener = ok %v, err %v", ok, err)
	}
	c, err := Dial("127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(c)

	var s int
	deadline := time.Now().Add(5 * time.Second)
	for {
		fd, ok, err := TryAccept(l)
		if err != nil {
			t.Fatalf("TryAccept: %v", err)
		}
		if ok {
			s = fd
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("TryAccept never saw the connection")
		}
		time.Sleep(time.Millisecond)
	}
	defer Close(s)

	if n, ok, err := TryRead(s, make([]byte, 1)); err != nil || ok || n != 0 {
		t.Fatalf("TryRead before write = %d, %v, %v", n, ok, err)
	}
	if n, ok, err := TryWrite(c, []byte("x")); err != nil || !ok || n != 1 {
		t.Fatalf("TryWrite = %d, %v, %v", n, ok, err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		buf := make([]byte, 1)
		n, ok, err := TryRead(s, buf)
		if err != nil {
			t.Fatalf("TryRead: %v", err)
		}
		if ok {
			if n != 1 || buf[0] != 'x' {
				t.Fatalf("TryRead = %d %q", n, buf[:n])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("TryRead never saw the byte")
		}
		time.Sleep(time.Millisecond)
	}
}

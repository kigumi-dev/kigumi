package driver_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

// TestHTTPSClientStalledTLSPeerTimesOut proves the HTTP deadline reaches the
// TLS session's transport loop: the peer completes the handshake and consumes
// the request, then answers nothing. A deadline that only bounded Wire.fill's
// outer loop would leave the caller suspended in tls.Session.read forever.
func TestHTTPSClientStalledTLSPeerTimesOut(t *testing.T) {
	t.Parallel()
	root, leaf := testTLSChain(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{leaf},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					line, err := br.ReadString('\n')
					if err != nil {
						return
					}
					if line == "\r\n" {
						break
					}
				}
				// The handshake and the request are done; never write a
				// response. This read ends when the client gives up.
				_, _ = br.ReadByte()
			}(conn)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	src := fmt.Sprintf(`import hex from std/encoding/hex
import http from std/net/http
import task from std/task
import time from std/time
import x509 from std/crypto/x509

const ROOT = "%s"

fn random(n: usize) -> Bytes! {
    host.entropy().bytes(n)
}

async fn check() -> Void! {
    let root = (x509.parse(hex.decode(ROOT).expect("hex"))).expect("root")
    let mut c = http.TlsClient.new(host.net(), task.BlockingPool.new(1), Array.of(root), random)
    c.timeoutMillis = 150
    let mut s = (await c.open("https://localhost:%d/"))?
    // Only the response read is timed: the handshake and the request already
    // finished, so the elapsed bound below covers the stall.
    let started = time.monotonic()
    match await s.send(http.Request.new("GET", "/")) {
        Ok(_) -> print "stalled response: unexpected"
        Err(e) -> print "stalled response: ${e.message()}"
    }
    if started.elapsedMillis() > 2000 {
        print "deadline not applied"
    }
}

task.currentThreadExecutor().runToCompletion(check())?
`, hex.EncodeToString(root), port)
	dir := t.TempDir()
	writeTestModule(t, dir, src)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	want := "stalled response: request timed out\n"

	type result struct {
		code         int
		out, errText string
		err          error
	}
	done := make(chan result, 1)
	go func() {
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
		done <- result{code: code, out: out.String(), errText: errOut.String(), err: err}
	}()
	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("vm run: %v\n%s", got.err, got.errText)
		}
		if got.code != 0 || got.out != want {
			t.Fatalf("vm exit %d, stdout %q, want %q, stderr:\n%s", got.code, got.out, want, got.errText)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("vm run did not return after the TLS peer stalled")
	}

	if testCompiler().CCompiler() == nil {
		t.Log("no C compiler: native stalled-peer run skipped")
		return
	}
	exe := filepath.Join(t.TempDir(), "tls-stall")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe)
	cmd.Dir = t.TempDir()
	binOut, err := cmd.Output()
	if ctx.Err() != nil {
		t.Fatal("native run did not return after the TLS peer stalled")
	}
	if err != nil {
		t.Fatalf("native run: %v", err)
	}
	if string(binOut) != want {
		t.Fatalf("native stdout %q, want %q", binOut, want)
	}
}

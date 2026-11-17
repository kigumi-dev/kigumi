package driver_test

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestHTTPSClientFramingOverTLS(t *testing.T) {
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
	replies := []string{
		"HTTP/1.1 304 Not Modified\r\nContent-Encoding: gzip\r\nContent-Length: 1234\r\n\r\n",
		"HTTP/1.1 200 OK\r\nContent-Encoding: deflate\r\nContent-Length: 0\r\n\r\n",
		"HTTP/1.0 200 OK\r\nContent-Length: 2\r\n\r\nok",
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		for _, reply := range replies {
			for {
				line, err := br.ReadString('\n')
				if err != nil {
					return
				}
				if line == "\r\n" {
					break
				}
			}
			if _, err := conn.Write([]byte(reply)); err != nil {
				return
			}
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	src := fmt.Sprintf(`import hex from std/encoding/hex
import http from std/net/http
import task from std/task
import x509 from std/crypto/x509

const ROOT = "%s"

fn random(n: usize) -> Bytes! {
    host.entropy().bytes(n)
}

fn show(r: http.Response!) {
    match r {
        Ok(x) -> print "${x.status} [${String.fromBytes(x.body) || "?"}]"
        Err(e) -> print "err ${e.message()}"
    }
}

async fn check() -> Void! {
    let root = (x509.parse(hex.decode(ROOT).expect("hex"))).expect("root")
    let c = http.TlsClient.new(host.net(), task.BlockingPool.new(1), Array.of(root), random)
    let mut s = (await c.open("https://localhost:%d/"))?
    show(await s.send(http.Request.new("GET", "/")))
    show(await s.send(http.Request.new("GET", "/")))
    show(await s.send(http.Request.new("GET", "/")))
    show(await s.send(http.Request.new("GET", "/")))
}

task.currentThreadExecutor().runToCompletion(check())?
`, hex.EncodeToString(root), port)
	dir := t.TempDir()
	writeTestModule(t, dir, src)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
	if err != nil {
		t.Fatalf("run: %v\n%s", err, errOut.String())
	}
	want := "304 []\n200 []\n200 [ok]\nerr connection closed before the response\n"
	if code != 0 || out.String() != want {
		t.Fatalf("exit %d, stdout %q, want %q, stderr:\n%s", code, out.String(), want, errOut.String())
	}
}

package driver_test

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestHTTPSClientRejectsUnknownRoot proves that a certificate is not accepted
// merely because it parses: a valid chain under a different root must fail.
func TestHTTPSClientRejectsUnknownRoot(t *testing.T) {
	t.Parallel()
	_, leaf := testTLSChain(t)
	otherRoot, _ := testTLSChain(t)
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
			go func() {
				defer conn.Close()
				br := bufio.NewReader(conn)
				for {
					line, err := br.ReadString('\n')
					if err != nil || line == "\r\n" {
						return
					}
				}
			}()
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

async fn check() -> Void! {
    let root = (x509.parse(hex.decode(ROOT).expect("hex"))).expect("root")
    let c = http.TlsClient.new(host.net(), task.BlockingPool.new(1), Array.of(root), random)
    match await c.get("https://localhost:%d/") {
        Ok(_) -> print "unexpected"
        Err(e) -> print e.message()
    }
}

task.currentThreadExecutor().runToCompletion(check())?
`, hex.EncodeToString(otherRoot), port)
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
	if code != 0 || !strings.HasPrefix(out.String(), "tls:") {
		t.Fatalf("exit %d, stdout %q, stderr:\n%s", code, out.String(), errOut.String())
	}
}

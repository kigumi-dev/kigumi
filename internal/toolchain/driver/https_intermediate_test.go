package driver_test

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

// TestHTTPSClientIntermediateChain accepts a server that sends a leaf and an
// intermediate while the client trusts only the root.
func TestHTTPSClientIntermediateChain(t *testing.T) {
	t.Parallel()
	rootDER, leaf := intermediateChain(t)
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
						break
					}
				}
				_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok"))
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
    let r = (await c.get("https://localhost:%d/"))?
    print "${r.status} ${String.fromBytes(r.body) || "?"}"
}

task.currentThreadExecutor().runToCompletion(check())?
`, hex.EncodeToString(rootDER), port)
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
	if code != 0 || out.String() != "200 ok\n" {
		t.Fatalf("exit %d, stdout %q, stderr:\n%s", code, out.String(), errOut.String())
	}
}

func intermediateChain(t *testing.T) ([]byte, tls.Certificate) {
	t.Helper()
	now := time.Now()
	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	root := &x509.Certificate{SerialNumber: big.NewInt(11), Subject: pkix.Name{CommonName: "root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	rootDER, _ := x509.CreateCertificate(rand.Reader, root, root, &rootKey.PublicKey, rootKey)

	intKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	inter := &x509.Certificate{SerialNumber: big.NewInt(12), Subject: pkix.Name{CommonName: "intermediate"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	intDER, _ := x509.CreateCertificate(rand.Reader, inter, root, &intKey.PublicKey, rootKey)

	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(13), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	leafDER, _ := x509.CreateCertificate(rand.Reader, leaf, inter, &leafKey.PublicKey, intKey)
	cert := tls.Certificate{
		Certificate: [][]byte{leafDER, intDER},
		PrivateKey:  leafKey,
	}
	return rootDER, cert
}

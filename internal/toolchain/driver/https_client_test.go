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
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

// TestHTTPSClientTalksToAGoServer exercises the whole std/net/http TlsClient
// path: URL parsing, the TLS handshake, certificate validation against a
// supplied root, an HTTP request over application data, and the response.
func TestHTTPSClientTalksToAGoServer(t *testing.T) {
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
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		for {
			line, err := br.ReadString('\n')
			if err != nil || line == "\r\n" {
				break
			}
		}
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nhello"))
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
	if code != 0 || out.String() != "200 hello\n" {
		t.Fatalf("exit %d, stdout %q, stderr:\n%s", code, out.String(), errOut.String())
	}
	<-done
}

func writeTestModule(t *testing.T, dir, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"httpsclient\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main", "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testTLSChain makes a P-256 root and a localhost leaf signed by it.
func testTLSChain(t *testing.T) ([]byte, tls.Certificate) {
	t.Helper()
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "kigumi test root"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return caDER, cert
}

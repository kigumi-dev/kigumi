package driver_test

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

// tlsInteropSrc connects to a local `openssl s_server`, validates its chain
// against a root supplied through TLS_ROOT, sends one HTTP/1.1 request and
// prints the ALPN protocol, status line and whether the page body arrived.
const tlsInteropSrc = `import fs from std/fs
import net from std/net
import task from std/task
import text from std/text
import tls from std/net/tls
import x509 from std/crypto/x509

async fn run() -> Void! {
    let rootPath = host.env("TLS_ROOT").expect("TLS_ROOT is set")
    let portText = host.env("TLS_PORT").expect("TLS_PORT is set")
    let port = text.parseInt(portText).expect("TLS_PORT is a number")
    let files = host.files()
    let io = task.BlockingPool.new(1)
    let rootFilePath = fs.Path.fromString(rootPath)?
    let rootText = (await io.run(() => files.read(rootFilePath)).unwrap())?
    let anchors = (x509.parsePem(rootText))?
    let n = host.net()
    let pool = task.BlockingPool.new(1)
    let mut conn = (await pool.run(() => n.connect("127.0.0.1", port)).unwrap())?
    let mut config = tls.Config.new("localhost", anchors, (k) => host.entropy().bytes(k))
    let mut session = (await tls.Client.connect(conn, config))?
    print "alpn ${session.alpn()}"
    (await session.write("GET / HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n".toBytes()))?
    let mut got = Bytes.zeros(0)
    let mut tries: usize = 0
    for tries < 8 && got.len() < 16 {
        match await session.read(16384) {
            Ok(chunk) -> {
                if chunk.len() == 0 {
                    break
                }
                got = got.concat(chunk)
            }
            Err(e) -> {
                if got.len() > 0 {
                    break
                }
                fail e
            }
        }
        tries = tries + 1
    }
    let page = String.fromBytes(got) || ""
    let firstLine = page.split("\r\n").get(0) || ""
    print "status ${firstLine}"
    print "page ${page.contains("s_server")}"
    await session.close()
    print "closed"
}

task.currentThreadExecutor().runToCompletion(run())?
`

func tlsRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("openssl", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("openssl %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// tlsCertificates writes a root CA plus one leaf per key type into dir and
// returns the root PEM path and the leaf certificate/key pairs.
func tlsCertificates(t *testing.T, dir string) (string, [][2]string) {
	t.Helper()
	rootKey := filepath.Join(dir, "root.key")
	rootPem := filepath.Join(dir, "root.pem")
	tlsRun(t, "ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", rootKey)
	tlsRun(t, "req", "-new", "-x509", "-key", rootKey, "-out", rootPem, "-days", "30",
		"-subj", "/CN=Kigumi Interop Root",
		"-addext", "basicConstraints=critical,CA:TRUE",
		"-addext", "keyUsage=critical,keyCertSign,cRLSign")

	ext := filepath.Join(dir, "leaf.ext")
	if err := os.WriteFile(ext, []byte("basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	type keySpec struct {
		name string
		gen  []string
	}
	specs := []keySpec{
		{"ec", []string{"ecparam", "-name", "prime256v1", "-genkey", "-noout", "-out", ""}},
		{"rsa", []string{"genpkey", "-algorithm", "RSA", "-pkeyopt", "rsa_keygen_bits:2048", "-out", ""}},
	}
	var pairs [][2]string
	for _, spec := range specs {
		base := filepath.Join(dir, spec.name)
		key := base + ".key"
		csr := base + ".csr"
		cert := base + ".pem"
		gen := append([]string{}, spec.gen...)
		gen[len(gen)-1] = key
		tlsRun(t, gen...)
		tlsRun(t, "req", "-new", "-key", key, "-out", csr, "-subj", "/CN=localhost")
		tlsRun(t, "x509", "-req", "-in", csr, "-CA", rootPem, "-CAkey", rootKey,
			"-CAcreateserial", "-out", cert, "-days", "30", "-sha256", "-extfile", ext)
		pairs = append(pairs, [2]string{cert, key})
	}
	return rootPem, pairs
}

// tlsFreePort asks the kernel for an unused loopback port, then releases it
// for openssl to bind.
func tlsFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func tlsWaitReady(t *testing.T, port int, stderr *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("openssl s_server did not listen on %d\n%s", port, stderr.String())
}

// TestTLSOpenSSLInterop runs the real client against OpenSSL's TLS 1.3
// server restricted to TLS_CHACHA20_POLY1305_SHA256, once with an ECDSA
// P-256 certificate and once with an RSA certificate, so both
// CertificateVerify schemes are exercised. It skips when openssl is absent.
func TestTLSOpenSSLInterop(t *testing.T) {
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl is not installed")
	}
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	certDir := t.TempDir()
	rootPem, pairs := tlsCertificates(t, certDir)

	root, err := os.MkdirTemp("", "kigumi-tls-interop")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if err := os.MkdirAll(filepath.Join(root, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(tlsInteropSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	if d := m.Diagnostics(); d != "" {
		t.Fatalf("unexpected diagnostics:\n%s", d)
	}

	for _, pair := range pairs {
		pair := pair
		name := strings.TrimSuffix(filepath.Base(pair[0]), ".pem")
		t.Run(name, func(t *testing.T) {
			port := tlsFreePort(t)
			var stderr bytes.Buffer
			cmd := exec.Command("openssl", "s_server",
				"-accept", strconv.Itoa(port),
				"-tls1_3",
				"-ciphersuites", "TLS_CHACHA20_POLY1305_SHA256",
				"-cert", pair[0],
				"-key", pair[1],
				"-alpn", "http/1.1",
				"-www", "-quiet")
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatalf("start openssl: %v", err)
			}
			defer func() {
				cmd.Process.Kill()
				cmd.Wait()
			}()
			tlsWaitReady(t, port, &stderr)

			t.Setenv("TLS_ROOT", rootPem)
			t.Setenv("TLS_PORT", strconv.Itoa(port))
			var out, errOut bytes.Buffer
			code, runErr := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir()}, &out, &errOut, []string{"main/main.kg"})
			if runErr != nil || code != 0 {
				t.Fatalf("run: err %v exit %d\nstdout: %s\nstderr: %s\nopenssl: %s",
					runErr, code, out.String(), errOut.String(), stderr.String())
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			if len(lines) != 4 {
				t.Fatalf("expected four lines, got %q", out.String())
			}
			if lines[0] != "alpn http/1.1" {
				t.Errorf("ALPN line = %q", lines[0])
			}
			if !strings.HasPrefix(lines[1], "status HTTP/1.0 200") && !strings.HasPrefix(lines[1], "status HTTP/1.1 200") {
				t.Errorf("status line = %q", lines[1])
			}
			if lines[2] != "page true" {
				t.Errorf("page line = %q", lines[2])
			}
			if lines[3] != "closed" {
				t.Errorf("close line = %q", lines[3])
			}
		})
	}
}

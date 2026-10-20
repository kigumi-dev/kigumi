package driver_test

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"kigumi/internal/driver"
)

const httpPostProbeSrc = `let h = host.net().http()
match h.post(host.args().get(1) || "?", "hi") {
    Ok(r) -> print "ok status=${r.status}"
    Err(e) -> print "err: ${e.message()}"
}
`

func buildHTTPProbe(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"httpprobe\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(httpPostProbeSrc), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	return exe
}

// TestHttpPostRequestLine checks the request line and Host header Http.post's native body sends, for a plain URL and an IPv6 literal.
func TestHttpPostRequestLine(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := buildHTTPProbe(t)
	cases := []struct {
		name, listenAddr, path, wantHost string
	}{
		{"ipv4", "127.0.0.1:0", "/foo/bar", ""},
		{"ipv6", "[::1]:0", "/v6path", "[::1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", c.listenAddr)
			if err != nil {
				t.Skipf("listen %s: %v", c.listenAddr, err)
			}
			defer ln.Close()
			addr := ln.Addr().(*net.TCPAddr)
			host := addr.IP.String()
			if addr.IP.To4() == nil {
				host = "[" + host + "]"
			}
			wantHost := c.wantHost
			if wantHost == "" {
				wantHost = host
			}
			url := fmt.Sprintf("http://%s:%d%s", host, addr.Port, c.path)
			got := make(chan string, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					got <- ""
					return
				}
				defer conn.Close()
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				buf := make([]byte, 4096)
				n, _ := conn.Read(buf)
				conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
				got <- string(buf[:n])
			}()
			cmd := exec.Command(exe, url)
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v\n%s", err, errOut.String())
			}
			if strings.TrimSpace(out.String()) != "ok status=200" {
				t.Fatalf("client output: %s (stderr: %s)", out.String(), errOut.String())
			}
			wire := <-got
			wantLine := "POST " + c.path + " HTTP/1.1\r\n"
			if !strings.HasPrefix(wire, wantLine) {
				t.Errorf("request line: got %q, want prefix %q", wire, wantLine)
			}
			wantHostLine := "Host: " + wantHost + "\r\n"
			if !strings.Contains(wire, wantHostLine) {
				t.Errorf("host header: got %q, want to contain %q", wire, wantHostLine)
			}
		})
	}
}

// TestHttpPostInvalidUTF8StatusLine checks invalid UTF-8 in the status line fails clearly instead of readLine silently swallowing the bad bytes into "".
func TestHttpPostInvalidUTF8StatusLine(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := buildHTTPProbe(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))
		buf := make([]byte, 4096)
		conn.Read(buf)
		conn.Write([]byte("HTTP/1.1 200 O\xffK\r\nContent-Length: 0\r\n\r\n"))
	}()
	addr := ln.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://127.0.0.1:%d/x", addr.Port)
	cmd := exec.Command(exe, url)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v\n%s", err, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "err: invalid UTF-8" {
		t.Fatalf("client output: got %q, want %q (stderr: %s)", got, "err: invalid UTF-8", errOut.String())
	}
}

// TestHttpPostTimeout checks that Http.post gives up against a peer that
// accepts the connection but never answers, instead of hanging forever.
func TestHttpPostTimeout(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	t.Parallel() // ~10s of its own blocked recv(); let it overlap other tests
	exe := buildHTTPProbe(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			<-done
			conn.Close()
		}
	}()
	addr := ln.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://127.0.0.1:%d/x", addr.Port)
	cmd := exec.Command(exe, url)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)
	close(done)
	if runErr != nil {
		t.Fatalf("run: %v\n%s", runErr, errOut.String())
	}
	if elapsed > 30*time.Second {
		t.Fatalf("Http.post took %s, expected a bounded timeout well under that", elapsed)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "err:") {
		t.Fatalf("expected a timeout error, got %q", out.String())
	}
}

// TestHttpPostConnectTimeout checks Http.post bounds connect() itself: a full accept queue drops the SYN, which a plain blocking connect() wouldn't catch for over a minute.
func TestHttpPostConnectTimeout(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	t.Parallel() // ~10s of its own blocked connect(); let it overlap other tests
	exe := buildHTTPProbe(t)
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		t.Fatal(err)
	}
	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	if err := unix.Listen(fd, 1); err != nil {
		t.Fatal(err)
	}
	sa, err := unix.Getsockname(fd)
	if err != nil {
		t.Fatal(err)
	}
	port := sa.(*unix.SockaddrInet4).Port

	// Fills the one-slot accept queue without ever calling accept(2), so
	// the probe's own connect() finds no room and its SYN is dropped.
	filler, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer filler.Close()

	url := fmt.Sprintf("http://127.0.0.1:%d/x", port)
	cmd := exec.Command(exe, url)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)
	if runErr != nil {
		t.Fatalf("run: %v\n%s", runErr, errOut.String())
	}
	if elapsed > 30*time.Second {
		t.Fatalf("Http.post took %s to give up on an unacknowledged connect, expected a bounded timeout well under that", elapsed)
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "err:") {
		t.Fatalf("expected a connect-timeout error, got %q", out.String())
	}
}

// runEmulatedEngine runs the HTTP post probe on the VM, buildHTTPProbe's in-process counterpart.
func runEmulatedEngine(t *testing.T, url string) string {
	t.Helper()
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"httpprobe\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(httpPostProbeSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if _, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main", url}); err != nil {
		t.Fatalf("vm: %v\n%s", err, errOut.String())
	}
	return strings.TrimSpace(out.String())
}

// TestHttpPostSlowDripSucceeds checks the 10s timeout is re-armed per read, not an overall deadline: a byte-at-a-time peer clears 10s total but the VM still succeeds.
func TestHttpPostSlowDripSucceeds(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	t.Parallel()
	exe := buildHTTPProbe(t)
	const line = "HTTP/1.1 200 \r\n"
	drip := func(t *testing.T) string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			buf := make([]byte, 4096)
			conn.Read(buf)
			for i := range len(line) {
				conn.Write([]byte{line[i]})
				time.Sleep(time.Second)
			}
		}()
		return fmt.Sprintf("http://%s/x", ln.Addr())
	}

	t.Run("native", func(t *testing.T) {
		t.Parallel()
		url := drip(t)
		cmd := exec.Command(exe, url)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\n%s", err, errOut.String())
		}
		if got, want := strings.TrimSpace(out.String()), "ok status=200"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
	t.Run("vm", func(t *testing.T) {
		t.Parallel()
		if got, want := runEmulatedEngine(t, drip(t)), "ok status=200"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

// TestHttpPostGracefulCloseBeforeStatusLine checks a peer that drains the request then closes without writing: an orderly EOF must end the read loop like native's got==0, not surface as a raw "EOF".
func TestHttpPostGracefulCloseBeforeStatusLine(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	t.Parallel()
	exe := buildHTTPProbe(t)
	const wantMsg = "err: malformed response"
	closeAfterDrain := func(t *testing.T) string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			buf := make([]byte, 4096)
			for {
				conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
				n, err := conn.Read(buf)
				if n == 0 || err != nil {
					break
				}
			}
		}()
		return fmt.Sprintf("http://%s/x", ln.Addr())
	}

	t.Run("native", func(t *testing.T) {
		t.Parallel()
		url := closeAfterDrain(t)
		cmd := exec.Command(exe, url)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\n%s", err, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != wantMsg {
			t.Fatalf("got %q, want %q", got, wantMsg)
		}
	})
	t.Run("vm", func(t *testing.T) {
		t.Parallel()
		if got := runEmulatedEngine(t, closeAfterDrain(t)); got != wantMsg {
			t.Fatalf("got %q, want %q", got, wantMsg)
		}
	})
}

// TestHttpPostResetAfterRequestSent checks a peer that closes with SO_LINGER{0} after reading (a deterministic TCP RST): the VM must report native's ECONNRESET strerror text, not a raw Go error.
func TestHttpPostResetAfterRequestSent(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	t.Parallel()
	exe := buildHTTPProbe(t)
	const wantMsg = "err: Connection reset by peer"
	resetAfterRead := func(t *testing.T) string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ln.Close() })
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			tc := conn.(*net.TCPConn)
			buf := make([]byte, 4096)
			tc.Read(buf)
			tc.SetLinger(0)
			tc.Close()
		}()
		return fmt.Sprintf("http://%s/x", ln.Addr())
	}

	t.Run("native", func(t *testing.T) {
		t.Parallel()
		url := resetAfterRead(t)
		cmd := exec.Command(exe, url)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\n%s", err, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != wantMsg {
			t.Fatalf("got %q, want %q", got, wantMsg)
		}
	})
	t.Run("vm", func(t *testing.T) {
		t.Parallel()
		if got := runEmulatedEngine(t, resetAfterRead(t)); got != wantMsg {
			t.Fatalf("got %q, want %q", got, wantMsg)
		}
	})
}

// TestHttpPostStalledTimeoutMessage checks a peer that never answers fails at ~10s with the same wording (native's EAGAIN strerror text) on the VM too.
func TestHttpPostStalledTimeoutMessage(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	t.Parallel()
	exe := buildHTTPProbe(t)
	const wantMsg = "err: Resource temporarily unavailable"
	stall := func(t *testing.T) string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		t.Cleanup(func() { close(done); ln.Close() })
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			<-done
			conn.Close()
		}()
		return fmt.Sprintf("http://%s/x", ln.Addr())
	}
	checkElapsed := func(t *testing.T, elapsed time.Duration) {
		t.Helper()
		if elapsed < 5*time.Second || elapsed > 30*time.Second {
			t.Fatalf("timed out after %s, expected close to the 10s per-call budget", elapsed)
		}
	}

	t.Run("native", func(t *testing.T) {
		t.Parallel()
		url := stall(t)
		cmd := exec.Command(exe, url)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		start := time.Now()
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\n%s", err, errOut.String())
		}
		checkElapsed(t, time.Since(start))
		if got := strings.TrimSpace(out.String()); got != wantMsg {
			t.Fatalf("got %q, want %q", got, wantMsg)
		}
	})
	t.Run("vm", func(t *testing.T) {
		t.Parallel()
		url := stall(t)
		start := time.Now()
		got := runEmulatedEngine(t, url)
		checkElapsed(t, time.Since(start))
		if got != wantMsg {
			t.Fatalf("got %q, want %q", got, wantMsg)
		}
	})
}

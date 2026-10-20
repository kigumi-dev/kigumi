package driver_test

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kigumi/internal/driver"
)

// TestHttpPostEngineMatrix checks the VM agrees with native on the same local listener.
func TestHttpPostEngineMatrix(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"httpprobe\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(httpPostProbeSrc), 0o644)

	runProbe := func(t *testing.T, url string) string {
		t.Helper()
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer
		if _, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main", url}); err != nil {
			t.Fatalf("vm: %v\n%s", err, errOut.String())
		}
		return strings.TrimSpace(out.String())
	}

	t.Run("success", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
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
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				buf := make([]byte, 4096)
				conn.Read(buf)
				conn.Write([]byte("HTTP/1.1 201 Created\r\nContent-Length: 0\r\n\r\n"))
				conn.Close()
			}
		}()
		addr := ln.Addr().(*net.TCPAddr)
		url := fmt.Sprintf("http://127.0.0.1:%d/x", addr.Port)
		if got, want := runProbe(t, url), "ok status=201"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("invalid utf8 status line", func(t *testing.T) {
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
			conn.Write([]byte("HTTP/1.1 20\xff Created\r\nContent-Length: 0\r\n\r\n"))
		}()
		addr := ln.Addr().(*net.TCPAddr)
		url := fmt.Sprintf("http://127.0.0.1:%d/x", addr.Port)
		if got, want := runProbe(t, url), "err: invalid UTF-8"; got != want {
			t.Errorf("vm: got %q, want %q", got, want)
		}
	})

	t.Run("malformed status line", func(t *testing.T) {
		cases := []string{
			"HTTP/1.1 20a OK\r\nContent-Length: 0\r\n\r\n",
			"HTTP/1.1 2000 OK\r\nContent-Length: 0\r\n\r\n",
			"NOTHTTP 200 OK\r\nContent-Length: 0\r\n\r\n",
		}
		for _, body := range cases {
			t.Run(body, func(t *testing.T) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer ln.Close()
				serve := func() {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					defer conn.Close()
					conn.SetDeadline(time.Now().Add(5 * time.Second))
					buf := make([]byte, 4096)
					conn.Read(buf)
					conn.Write([]byte(body))
				}
				addr := ln.Addr().(*net.TCPAddr)
				url := fmt.Sprintf("http://127.0.0.1:%d/x", addr.Port)
				go serve()
				if got, want := runProbe(t, url), "err: malformed response"; got != want {
					t.Errorf("got %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("does not follow redirects", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		addr := ln.Addr().(*net.TCPAddr)
		var requests atomic.Int32
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				requests.Add(1)
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				buf := make([]byte, 4096)
				conn.Read(buf)
				loc := fmt.Sprintf("http://127.0.0.1:%d/redirected", addr.Port)
				conn.Write([]byte("HTTP/1.1 302 Found\r\nLocation: " + loc + "\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
				conn.Close()
			}
		}()
		url := fmt.Sprintf("http://127.0.0.1:%d/x", addr.Port)
		if got, want := runProbe(t, url), "ok status=302"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if n := requests.Load(); n != 1 {
			t.Errorf("got %d requests, want 1 (no redirect followed)", n)
		}
	})

	t.Run("validation error matches native wording", func(t *testing.T) {
		if got, want := runProbe(t, "https://example.com/x"), "err: https is not supported in native builds"; got != want {
			t.Errorf("vm: got %q, want %q", got, want)
		}
	})
}

package driver_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

const httpRawServerSrc = `import http from std/net/http
import net from std/net
import task from std/task

fn echo(req: http.Request) -> http.Response! {
    http.Response.ofText(200, "echo ${req.method} ${req.path} ${req.body.len()}")
}

async fn watch(control: net.Listener, stop: http.Shutdown) -> Void! {
    let mut control = control
    for true {
        if control.pollAccept()? is Some(_) {
            stop.request()
            return
        }
        await task.yieldNow()
    }
}

async fn root(listener: net.Listener, control: net.Listener) -> Void! {
    let mut server = http.Server.new(http.Router.new().get("/", echo).post("/", echo).handler(), (line) => print "log: ${line}")
    server.limits = http.Limits {
        maxHeaderBytes: 1024
        maxHeaders: 4
        maxBodyBytes: 64
        readTimeoutMillis: 400
        idleTimeoutMillis: 400
        writeTimeoutMillis: 5000
        maxConnections: 8
    }
    let stop = http.Shutdown.new()
    let watcher = task.spawn(watch(control, stop.clone()))
    (await server.serve(listener, stop))?
    (await watcher)?
}

let n = host.net()
let listener = n.listen("127.0.0.1", 0)?
let control = n.listen("127.0.0.1", 0)?
print "ports ${listener.port()} ${control.port()}"
task.currentThreadExecutor().runToCompletion(root(listener, control))?
print "stopped"
`

// httpRawCases are requests the std/net/http client refuses to produce; each is sent on its own connection.
var httpRawCases = []struct{ name, send, want string }{
	{"nul in value", "GET / HTTP/1.1\r\nHost: t\r\nX: a\x00b\r\n\r\n", "HTTP/1.1 400 "},
	{"obs text", "GET / HTTP/1.1\r\nHost: t\r\nX: \xff\xfe\r\n\r\n", "HTTP/1.1 200 "},
	{"too many fields", "GET / HTTP/1.1\r\nHost: t\r\nA: 1\r\nB: 2\r\nC: 3\r\nD: 4\r\n\r\n", "HTTP/1.1 431 "},
	{"space in host", "GET / HTTP/1.1\r\nHost: a b\r\n\r\n", "HTTP/1.1 400 "},
	{"unbracketed ipv6 host", "GET / HTTP/1.1\r\nHost: a:b:c\r\n\r\n", "HTTP/1.1 400 "},
	{"chunk size overflow", "POST / HTTP/1.1\r\nHost: t\r\nTransfer-Encoding: chunked\r\n\r\nfffffffffffffffff\r\n", "HTTP/1.1 400 "},
	{"negative length", "POST / HTTP/1.1\r\nHost: t\r\nContent-Length: -1\r\n\r\n", "HTTP/1.1 400 "},
	{"list length", "POST / HTTP/1.1\r\nHost: t\r\nContent-Length: 1, 2\r\n\r\nab", "HTTP/1.1 400 "},
	{"duplicate length", "POST / HTTP/1.1\r\nHost: t\r\nContent-Length: 1\r\nContent-Length: 2\r\n\r\nab", "HTTP/1.1 400 "},
	{"leading zero length", "POST / HTTP/1.1\r\nHost: t\r\nContent-Length: 0000000000000003\r\nConnection: close\r\n\r\nabc", "HTTP/1.1 200 "},
	{"absolute form", "GET http://t/ HTTP/1.1\r\nHost: t\r\n\r\n", "HTTP/1.1 400 "},
	{"tab in request line", "GET\t/ HTTP/1.1\r\nHost: t\r\n\r\n", "HTTP/1.1 400 "},
	{"slow head", "GET / HTTP/1.1\r\nHost: t\r\n", "HTTP/1.1 408 "},
	{"idle", "", ""},
	{"garbage then request", "GARBAGE\r\n\r\nGET / HTTP/1.1\r\nHost: t\r\n\r\n", "HTTP/1.1 400 "},
	{"short body then half close", "POST / HTTP/1.1\r\nHost: t\r\nContent-Length: 10\r\n\r\nabc", "HTTP/1.1 400 "},
}

// TestHttpServerRawMalformed sends raw requests to a std/net/http server on both backends and checks each answer and that the connection closes after it.
func TestHttpServerRawMalformed(t *testing.T) {
	if os.Getenv("KIGUMI_HTTP_RAW_ROLE") == "vm" {
		runHTTPRawVM(t)
		return
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := writeHTTPRawModule(t)
	exe := filepath.Join(root, "prog")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	vm := exec.Command(os.Args[0], "-test.run=^TestHttpServerRawMalformed$", "-test.count=1")
	vm.Env = append(os.Environ(), "KIGUMI_HTTP_RAW_ROLE=vm", "KIGUMI_HTTP_RAW_ROOT="+root)
	for _, c := range []struct {
		name string
		cmd  *exec.Cmd
	}{{"native", exec.Command(exe)}, {"vm", vm}} {
		t.Run(c.name, func(t *testing.T) { checkHTTPRaw(t, c.cmd) })
	}
}

func writeHTTPRawModule(t *testing.T) string {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"httpraw\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(httpRawServerSrc), 0o644)
	return root
}

func runHTTPRawVM(t *testing.T) {
	m, err := driver.LoadModule(os.Getenv("KIGUMI_HTTP_RAW_ROOT"), driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	code, err := driver.Run(m, os.Stdout, os.Stderr, nil)
	if err != nil || code != 0 {
		t.Fatalf("vm: exit %d, %v", code, err)
	}
}

func checkHTTPRaw(t *testing.T, cmd *exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	lines := bufio.NewScanner(stdout)
	var port, control int
	for lines.Scan() {
		if _, err := fmt.Sscanf(lines.Text(), "ports %d %d", &port, &control); err == nil {
			break
		}
	}
	if port == 0 {
		t.Fatalf("no ports line\nstderr:\n%s", errOut.String())
	}
	for _, c := range httpRawCases {
		got := rawHTTPExchange(t, port, c.send, c.name == "short body then half close")
		if !strings.HasPrefix(got, c.want) || (c.want == "" && got != "") {
			t.Errorf("%s: got %q, want prefix %q", c.name, got, c.want)
		}
		if c.want != "" && strings.Count(got, "HTTP/1.1 ") != 1 {
			t.Errorf("%s: want exactly one response before the close, got %q", c.name, got)
		}
	}
	stop, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", control))
	if err != nil {
		t.Fatal(err)
	}
	stop.Close()
	rest, _ := io.ReadAll(stdout)
	if err := cmd.Wait(); err != nil {
		t.Fatalf("server exit: %v\nstderr:\n%s", err, errOut.String())
	}
	if !strings.Contains(string(rest), "stopped") {
		t.Errorf("server did not report stopping: %q", rest)
	}
}

// rawHTTPExchange writes send and returns everything the server writes until it closes the connection.
func rawHTTPExchange(t *testing.T, port int, send string, halfClose bool) string {
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := conn.Write([]byte(send)); err != nil {
		t.Fatal(err)
	}
	if halfClose {
		conn.(*net.TCPConn).CloseWrite()
	}
	got, err := io.ReadAll(conn)
	if err != nil && !strings.Contains(err.Error(), "reset") {
		t.Fatalf("read: %v (got %q)", err, got)
	}
	return string(got)
}

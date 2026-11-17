package driver_test

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const defaultPortHostSrc = `import http from std/net/http
import net from std/net
import task from std/task

fn echo(req: http.Request) -> http.Response! {
    http.Response.ofText(200, "Host: ${req.headers.get("Host") || "-"}")
}

async fn root(n: net.Net) -> Void! {
    let listener = n.listen("127.0.0.1", 80)?
    let server = http.Server.new(http.Router.new().get("/", echo).handler(), (line) => print "log: ${line}")
    let stop = http.Shutdown.new()
    let running = task.spawn(server.serve(listener, stop.clone()))
    let client = http.Client.new(n, task.BlockingPool.new(1))
    for url in Array.of("http://127.0.0.1:/", "http://127.0.0.1:80/") {
        match await client.get(url) {
            Ok(r) -> print "${url} ${r.status} ${r.text()?}"
            Err(e) -> print "${url} err ${e.message()}"
        }
    }
    stop.request()
    (await running)?
}

task.currentThreadExecutor().runToCompletion(root(host.net()))?
`

// Port 80 needs privilege, so the program runs as root of its own user and network namespace.
func TestHTTPClientHostOmitsDefaultPort(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	unshare, err := exec.LookPath("unshare")
	if err != nil {
		t.Skip("no unshare")
	}
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skip("no ip")
	}
	if err := exec.Command(unshare, "-rn", "true").Run(); err != nil {
		t.Skipf("no unprivileged network namespace: %v", err)
	}
	dir := t.TempDir()
	writeTestModule(t, dir, defaultPortHostSrc)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "default-port-host")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{}); err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	var stderr bytes.Buffer
	cmd := exec.Command(unshare, "-rn", "sh", "-c", `ip link set lo up && exec "$0"`, exe)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, stderr.String())
	}
	want := "http://127.0.0.1:/ 200 Host: 127.0.0.1\nhttp://127.0.0.1:80/ 200 Host: 127.0.0.1\n"
	if string(out) != want {
		t.Fatalf("stdout %q, want %q", out, want)
	}
}

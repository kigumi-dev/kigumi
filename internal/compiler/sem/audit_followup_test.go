package sem_test

import (
	"strings"
	"testing"
)

func TestBlockingTerminalPrimitivesRejected(t *testing.T) {
	cases := map[string]string{
		"remove":     "async fn run(files: fs.Files, path: fs.Path) -> Void! { files.remove(path) }",
		"removeDir":  "async fn run(files: fs.Files, path: fs.Path) -> Void! { files.removeDir(path) }",
		"rename":     "async fn run(files: fs.Files, path: fs.Path) -> Void! { files.rename(path, path) }",
		"stat":       "async fn run(files: fs.Files, path: fs.Path) -> Void! { _ = files.stat(path)? }",
		"chmod":      "async fn run(files: fs.Files, path: fs.Path) -> Void! { files.chmod(path, 0o600) }",
		"createWith": "async fn run(files: fs.Files, path: fs.Path) -> Void! { _ = files.createWithMode(path, 0o600)? }",
		"sleep":      "async fn run() { time.sleep(1) }",
	}
	for name, body := range cases {
		src := "import fs from std/fs\nimport time from std/time\n" + body
		out := checkBlocking(t, "single-thread-event-loop", src).Render()
		if !strings.Contains(out, "E1320") {
			t.Errorf("%s: expected E1320, got:\n%s", name, out)
		}
	}
	if out := checkBlocking(t, "single-thread-event-loop", "import time from std/time\nasync fn run() -> i64 { time.monotonic().elapsedMillis() }").Render(); out != "" {
		t.Errorf("clock read should stay quiet, got:\n%s", out)
	}
}

func TestCallbackCalleeReachesOperationSummary(t *testing.T) {
	const callers = `
fn invoke(f: fn() -> Bool) -> Bool { f() }
fn identity(f: escaping fn() -> Bool) -> fn() -> Bool { f }
type Holder = { call fn() -> Bool }
fn Holder.invoke(self) -> Bool { (self.call)() }
fn make(f: escaping fn() -> Bool) -> Holder { Holder { call: f } }
fn makeResult(f: escaping fn() -> Bool) -> Holder! { Holder { call: f } }
`
	bodies := map[string]string{
		"direct":        "blocking()",
		"paren":         "(blocking)()",
		"field":         "let holder = Holder { call: blocking }\n holder.call()",
		"fieldParen":    "let holder = Holder { call: blocking }\n (holder.call)()",
		"fieldLocal":    "let holder = Holder { call: blocking }\n let call = holder.call\n call()",
		"argument":      "invoke(blocking)",
		"returned":      "let call = identity(blocking)\n call()",
		"receiver":      "let holder = Holder { call: blocking }\n holder.invoke()",
		"factory":       "let holder = make(blocking)\n (holder.call)()",
		"factoryResult": "let holder = makeResult(blocking)?\n (holder.call)()",
	}
	for name, body := range bodies {
		src := "import fs from std/fs\nlet marker = 0\nfn blocking() -> Bool { host.files().exists(fs.Path.fromString(\"/tmp\").unwrap()) }\n" + callers + "\nasync fn run() -> Bool! {\n" + body + "\n}"
		out := checkBlocking(t, "single-thread-event-loop", src).Render()
		if !strings.Contains(out, "E1320") {
			t.Errorf("%s: expected E1320, got:\n%s", name, out)
		}
	}
}

func TestDropSummaryTracksDestructorFixpoint(t *testing.T) {
	for _, layout := range []string{
		"type Guard = resource { files fs.Files }\nfn Guard.drop(move self) { _ = self.files.exists(fs.Path.fromString(\"/tmp\").unwrap()) }",
		"fn helper() -> Bool { host.files().exists(fs.Path.fromString(\"/tmp\").unwrap()) }\nfn early() -> Bool { helper() }\ntype Guard = resource { files fs.Files }\nfn Guard.drop(move self) { _ = early() }",
		"type Guard = resource { files fs.Files }\nfn Guard.drop(move self) { _ = late() }\nfn late() -> Bool { host.files().exists(fs.Path.fromString(\"/tmp\").unwrap()) }",
	} {
		src := "import fs from std/fs\n" + layout + "\nasync fn run(files: fs.Files) { let guard = Guard { files: files } }"
		out := checkBlocking(t, "single-thread-event-loop", src).Render()
		if !strings.Contains(out, "E1320") {
			t.Errorf("drop summary stale:\n%s\n%s", src, out)
		}
	}
}

func TestPureCallerMutationContract(t *testing.T) {
	reject := map[string]string{
		"scalar":  "pure fn bump(n: &mut Int) { n += 1 }\nlet mut n = 0\nbump(&mut n)",
		"array":   "pure fn change(a: &mut Array[Int]) { a.push(7) }\nlet mut a = Array.empty[Int]()\nchange(&mut a)",
		"builder": "import {StringBuilder} from std/text\npure fn change(b: &mut StringBuilder) { b.push(\"x\") }\nlet mut b = StringBuilder.new()\nchange(&mut b)",
		"self":    "type R = { pub mut value Int }\npure fn R.bump(mut self) { self.value += 1 }\nlet mut r = R { value: 0 }\nr.bump()",
		"helper":  "type R = { pub mut value Int }\nfn R.bump(mut self) { self.value += 1 }\npure fn change(r: &mut R) { r.bump() }\nlet mut r = R { value: 0 }\nchange(&mut r)",
	}
	for name, src := range reject {
		out := checkBlocking(t, "", src).Render()
		if !strings.Contains(out, "E904") {
			t.Errorf("%s: expected E904 caller-mutation, got:\n%s", name, out)
		}
	}
	accept := map[string]string{
		"local":   "pure fn f() -> Int { let mut v = 0\n v += 1\n v }",
		"builder": "import {StringBuilder} from std/text\npure fn f() -> String! { let mut b = StringBuilder.new()\n b.push(\"x\")\n b.freeze() }",
	}
	for name, src := range accept {
		if out := checkBlocking(t, "", src).Render(); out != "" {
			t.Errorf("%s: fresh local should stay pure, got:\n%s", name, out)
		}
	}
}

func TestHTTPHandlerCallbackReachesServe(t *testing.T) {
	const src = `
import fs from std/fs
import net from std/net
import http from std/net/http

let marker = 0

fn handler(req: http.Request) -> http.Response! {
    let path = fs.Path.fromString("/tmp/kigumi-blocking-probe")?
    let body = host.files().read(path)?
    http.Response.ofText(200, body)
}

async fn serve(listener: net.Listener, stop: http.Shutdown) -> Void! {
    let server = http.Server.new(handler, (line) => {})
    (await server.serve(listener, stop))?
}
`
	if out := checkBlocking(t, "single-thread-event-loop", src).Render(); !strings.Contains(out, "E1320") {
		t.Errorf("http handler callback should reach serve, got:\n%s", out)
	}
	if out := checkBlocking(t, "", src).Render(); !strings.Contains(out, "W1320") {
		t.Errorf("http handler callback should warn under hosted, got:\n%s", out)
	}
}

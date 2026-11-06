package sem_test

import (
	"strings"
	"testing"
)

func TestAsyncNonBlockingSocketsQuiet(t *testing.T) {
	res := checkBlocking(t, "single-thread-event-loop", `
import net from std/net
import task from std/task

async fn pump(l: net.Listener) -> Void! {
    let mut l = l
    let mut buf = Array.empty[u8]()
    for true {
        if l.pollAccept()? is Some(c) {
            let mut c = c
            _ = c.pollRead(&mut buf, 16)?
            _ = c.pollWrite("x".toBytes())?
            return
        }
        await task.yieldNow()
    }
}
`)
	if out := res.Render(); res.HasErrors() {
		t.Fatalf("the try* socket calls never block, so a strict profile must accept them:\n%s", out)
	}
}

func TestAsyncBlockingSocketReadStillRejected(t *testing.T) {
	res := checkBlocking(t, "single-thread-event-loop", `
import net from std/net

async fn pull(c: net.Conn) -> usize! {
    let mut c = c
    let mut buf = Array.empty[u8]()
    c.read(&mut buf, 16)
}
`)
	if out := res.Render(); !res.HasErrors() || !strings.Contains(out, "E1320") {
		t.Fatalf("Conn.read blocks and must still be rejected under a strict profile:\n%s", out)
	}
}

func TestAsyncHTTPServerQuiet(t *testing.T) {
	res := checkBlocking(t, "single-thread-event-loop", `
import http from std/net/http
import net from std/net
import task from std/task

fn ok(req: http.Request) -> http.Response! {
    http.Response.ofText(200, "ok")
}

async fn serve(l: net.Listener) -> Void! {
    let server = http.Server.new(http.Router.new().get("/", ok).handler(), (line) => {})
    let stop = http.Shutdown.new()
    let t = task.spawn(server.serve(l, stop.clone()))
    stop.request()
    (await t)?
}
`)
	if out := res.Render(); res.HasErrors() {
		t.Fatalf("std/net/http must not reach a direct-blocking call from its server's async bodies:\n%s", out)
	}
}

func TestAsyncHTTPClientConnectBlocksWithoutWorkers(t *testing.T) {
	res := checkBlocking(t, "single-thread-event-loop", `
import http from std/net/http
import net from std/net
import task from std/task

async fn fetch(n: net.Net) -> Void! {
    let c = http.Client.new(n, task.BlockingPool.new(1))
    _ = (await c.get("http://127.0.0.1:1/"))?
}
`)
	out := res.Render()
	if !strings.Contains(out, "E1320") || !strings.Contains(out, "calls `connect`") {
		t.Fatalf("without worker threads the client's pool runs connect on the event loop, so a strict profile must reject it:\n%s", out)
	}
}

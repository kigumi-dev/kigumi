package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const blockingAsyncSrc = `
import fs from std/fs

async fn readIt() -> String! {
    host.files().read(fs.Path.fromString("x")?)?
}
`

func checkBlocking(t *testing.T, profile, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	if main.Entry.HasErrors() {
		t.Fatalf("parse errors:\n%s", main.Entry.Dump())
	}
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{
		Packages:        append(testkit.LoadStd(t, "../../std"), main),
		BlockingProfile: profile,
	}, schema.GenerateDerive)
}

func TestAsyncDirectBlockStrictRejected(t *testing.T) {
	res := checkBlocking(t, "single-thread-event-loop", blockingAsyncSrc)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1320") {
		t.Fatalf("expected E1320 under a strict profile:\n%s", out)
	}
	if !strings.Contains(out, "calls `read`") && !strings.Contains(out, "calls `readBytes`") {
		t.Fatalf("expected a call-chain trace down to the blocking read, got:\n%s", out)
	}
}

func TestAsyncDirectBlockHostedWarns(t *testing.T) {
	res := checkBlocking(t, "", blockingAsyncSrc)
	out := res.Render()
	if res.HasErrors() {
		t.Fatalf("the hosted default must warn, not reject:\n%s", out)
	}
	if !strings.Contains(out, "W1320") {
		t.Fatalf("expected a W1320 warning, got:\n%s", out)
	}
}

func TestAsyncDirectBlockMixedOffloadedThenDirect(t *testing.T) {
	const src = `
import fs from std/fs
import task from std/task

async fn mixed() -> String! {
    let pool = task.BlockingPool.new(4)
    let fut = pool.run(() => 1).unwrap()
    _ = await fut
    host.files().read(fs.Path.fromString("x")?)?
}
`
	res := checkBlocking(t, "single-thread-event-loop", src)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1320") {
		t.Fatalf("an offloaded call reached first must not mask a later real direct-blocking call:\n%s", out)
	}
}

func TestAsyncDirectBlockOffloadedQuiet(t *testing.T) {
	res := checkBlocking(t, "single-thread-event-loop", `
import task from std/task

async fn useProvider() -> Result[Void, task.BlockingError] {
    let pool = task.BlockingPool.new(4)
    let fut = pool.run(() => 1)?
    _ = await fut
}
`)
	out := res.Render()
	if res.HasErrors() {
		t.Fatalf("offloaded blocking through the provider must not report:\n%s", out)
	}
}

package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const spawnSrc = `import task from std/task

async fn leaf(n: Int) -> Int {
    n
}

async fn root() -> Int {
    let t = task.spawn(leaf(1))
    await t
}

let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(root())}"
`

const noSpawnSrc = `import task from std/task

async fn leaf(n: Int) -> Int {
    n
}

let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(leaf(1))}"
`

const handoffChildSrc = `import task from std/task

async fn leaf(n: Int) -> Int {
    n
}

async fn root() -> Int {
    let other = task.currentThreadExecutor()
    let t = other.handoff(leaf(1))
    await t
}

let ex = task.currentThreadExecutor()
print "${ex.runToCompletion(root())}"
`

func TestSpawnWarnsWhereTaskPanicIsFatal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, src string
		target    driver.Target
		warn      bool
	}{
		{"wasi", spawnSrc, mustTarget(t, "wasm32-wasi", ""), true},
		{"wasm freestanding", spawnSrc, mustTarget(t, "wasm32-freestanding", ""), true},
		{"aarch64 bare", spawnSrc, mustTarget(t, "aarch64-linux-gnu", "none"), true},
		{"riscv64 bare", spawnSrc, mustTarget(t, "riscv64-linux-gnu", "none"), true},
		{"x86 bare contains it", spawnSrc, mustTarget(t, "x86_64-linux-gnu", "none"), false},
		{"hosted", spawnSrc, mustTarget(t, "", ""), false},
		{"wasi handoff", handoffChildSrc, mustTarget(t, "wasm32-wasi", ""), true},
		{"aarch64 bare handoff", handoffChildSrc, mustTarget(t, "aarch64-linux-gnu", "none"), true},
		{"hosted handoff", handoffChildSrc, mustTarget(t, "", ""), false},
		{"wasi without spawn", noSpawnSrc, mustTarget(t, "wasm32-wasi", ""), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			out, ok := compileFor(t, c.target, c.src)
			if !ok {
				t.Fatalf("build refused:\n%s", out)
			}
			got := strings.Count(out, "W1860")
			switch {
			case c.warn && got != 1:
				t.Fatalf("want one W1860, got %d:\n%s", got, out)
			case !c.warn && got != 0:
				t.Fatalf("unexpected W1860:\n%s", out)
			}
			if c.warn && (!strings.Contains(out, c.target.Triple) || !strings.Contains(out, "spawn(leaf(1))") && !strings.Contains(out, "handoff(leaf(1))")) {
				t.Errorf("warning must name the target %q and point at the spawn:\n%s", c.target.Triple, out)
			}
		})
	}
}

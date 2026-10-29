package sem_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

func TestRootBridgeForwardBindingsIsolateCallSites(t *testing.T) {
	const prelude = `import task from std/task
async fn tick() -> Int { 1 }
fn drive() -> Int { task.currentThreadExecutor().runToCompletion(tick()) }
fn safe() -> Int { 1 }
fn leaf(f: escaping fn() -> Int) -> Int { f() }
fn split(f: escaping fn() -> Int, g: escaping fn() -> Int) -> Int { leaf(f) + leaf(g) }
`
	for _, tc := range []struct {
		first, second string
		blocked       bool
	}{{"safe", "safe", false}, {"safe", "drive", true}, {"drive", "safe", true}, {"drive", "drive", true}} {
		t.Run(tc.first+"_"+tc.second, func(t *testing.T) {
			body := prelude + fmt.Sprintf("async fn work() -> Int { split(%s, %s) }\n", tc.first, tc.second)
			fx := fixture{sections: []section{{name: "src", body: body}}}
			r := sem.CheckWith(buildModule(t, fx), schema.GenerateDerive)
			if got := strings.Count(r.Render(), "error[E1291]"); (got == 1) != tc.blocked || got > 1 || r.HasErrors() != tc.blocked {
				t.Fatalf("blocked = %v, diagnostics:\n%s", tc.blocked, r.Render())
			}
		})
	}
}

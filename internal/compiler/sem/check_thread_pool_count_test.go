package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func checkPoolCount(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
}

// A thread pool's worker count has no runtime Err form — a bad count aborts
// the process — so `check` must reject a literal outside 1 to 256 instead of
// accepting it and failing on the user's machine.
func TestThreadPoolExecutorRejectsLiteralOutsideBounds(t *testing.T) {
	for _, count := range []string{"0", "257", "10000"} {
		t.Run(count, func(t *testing.T) {
			res := checkPoolCount(t, "import task from std/task\n\nasync fn root() -> Int { 1 }\nlet ex = task.threadPoolExecutor("+count+")\nprint \"${ex.runToCompletion(root())}\"\n")
			out := res.Render()
			if !res.HasErrors() || !strings.Contains(out, "E1463") {
				t.Fatalf("check accepted threadPoolExecutor(%s):\n%s", count, out)
			}
			if !strings.Contains(out, "outside 1 to 256") {
				t.Fatalf("E1463 lacks the bound:\n%s", out)
			}
		})
	}
}

// The bound is inclusive on both ends, and a non-literal count stays a run-time
// question: the checker cannot know it.
func TestThreadPoolExecutorAcceptsBoundsAndOpaqueCounts(t *testing.T) {
	for _, count := range []string{"1", "256", "workers"} {
		t.Run(count, func(t *testing.T) {
			res := checkPoolCount(t, "import task from std/task\n\nasync fn root() -> Int { 1 }\nfn start(workers: usize) -> task.Executor {\n    task.threadPoolExecutor("+count+")\n}\nlet ex = start(2)\nprint \"${ex.runToCompletion(root())}\"\n")
			if res.HasErrors() {
				t.Fatalf("check rejected a permitted count %s:\n%s", count, res.Render())
			}
		})
	}
}

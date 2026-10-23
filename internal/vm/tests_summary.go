package vm

import (
	"fmt"
	"io"
)

// NoTestsMatch is a --filter that selects no test, which fails the run instead of passing it vacuously.
type NoTestsMatch struct{ Filter string }

func (e *NoTestsMatch) Error() string { return fmt.Sprintf("no test matches --filter %q", e.Filter) }

type testFailure struct {
	id   string
	file string
	line int
}

func writeSummary(out io.Writer, passed, failed, skipped int, failures []testFailure) {
	head := "ok"
	if failed > 0 || skipped > 0 {
		head = "FAIL"
	}
	fmt.Fprintf(out, "%s: %d passed, %d failed, %d skipped\n", head, passed, failed, skipped)
	for _, f := range failures {
		fmt.Fprintf(out, "  failed %s (%s:%d)\n", f.id, f.file, f.line)
	}
}

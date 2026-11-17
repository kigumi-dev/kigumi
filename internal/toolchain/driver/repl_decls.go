package driver

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

// replDeclChunk is one appended piece of the session source and the cell that
// brought it in.
type replDeclChunk struct {
	src    string
	origin string
}

func (r *Repl) chunksFor(cell *replCell, needPrelude, needShell bool) []replDeclChunk {
	chunks := append([]replDeclChunk(nil), r.chunks...)
	if needPrelude && !r.prelude {
		chunks = append(chunks, replDeclChunk{src: replPrelude, origin: "the REPL prelude"})
	}
	if needShell && !r.shell {
		chunks = append(chunks, replDeclChunk{src: replShellPrelude, origin: "the REPL shell prelude"})
	}
	if cell.decls != "" {
		chunks = append(chunks, replDeclChunk{src: cell.decls, origin: fmt.Sprintf("cell %d", r.cells)})
	}
	return chunks
}

func joinDeclChunks(chunks []replDeclChunk) string {
	srcs := make([]string, len(chunks))
	for i, c := range chunks {
		srcs[i] = c.src
	}
	return strings.Join(srcs, "\n")
}

// declChunkAt returns the chunk holding the 1-based line of the joined source.
func declChunkAt(chunks []replDeclChunk, line int) (replDeclChunk, bool) {
	start := 1
	for _, c := range chunks {
		end := start + strings.Count(c.src, "\n") + 1
		if line >= start && line < end {
			return c, true
		}
		start = end
	}
	return replDeclChunk{}, false
}

// renderDeclDiag reports a diagnostic that lands in persisted declarations
// instead of the current cell, naming the declaration and the cell it came from.
func renderDeclDiag(res *sem.Result, chunks []replDeclChunk, line int, d diag.Diagnostic) string {
	body := diag.RenderAll(res, []diag.Diagnostic{d})
	c, ok := declChunkAt(chunks, line)
	if !ok {
		return body
	}
	name := "a declaration"
	if names := replDeclNames(c.src); len(names) > 0 {
		name = "`" + names[0] + "`"
	}
	return fmt.Sprintf("repl: %s from %s no longer builds:\n%s", name, c.origin, body)
}

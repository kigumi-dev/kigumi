package sem

import (
	"strings"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

// The driver passes the first spawn site and the target, which sem does not know.
func (r *Result) WarnTaskPanicFatal(target string, file FileID, node syntax.NodeID) {
	r.errAt(file, node, cTaskPanicFatal, target)
}

func (r *Result) RenderTaskPanicWarning() string {
	var sb strings.Builder
	for i := 1; i < len(r.Files); i++ {
		var ds []diag.Diagnostic
		for _, d := range r.Files[i].Diags {
			if d.Code == cTaskPanicFatal.Num {
				ds = append(ds, d)
			}
		}
		sb.WriteString(diag.RenderAll(r, ds))
	}
	return sb.String()
}

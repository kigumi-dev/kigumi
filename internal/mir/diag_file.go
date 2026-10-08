package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
)

type FileDiag struct {
	File sem.FileID
	Diag diag.Diagnostic
}

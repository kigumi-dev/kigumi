package ir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

type FileDiag struct {
	File sem.FileID
	Diag diag.Diagnostic
}

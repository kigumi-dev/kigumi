package syntax

import (
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// ErrorCode is the diagnostic code of every syntax error; the message carries the detail.
const ErrorCode = "E2199"

func syntaxError(span token.Span, msg string) diag.Diagnostic {
	d := diag.Errorf(diag.Location{Span: span}, msg)
	d.Code = ErrorCode
	return d
}

package driver_test

import (
	"testing"
	"time"
)

// The status-line table must cover the standard codes clients classify on;
// 206 and 416 were returning an empty reason while 200 returned "OK".
func TestHTTPReasonPhrases(t *testing.T) {
	const src = `import http from std/net/http
print "${http.reason(200)}|${http.reason(203)}|${http.reason(206)}|${http.reason(416)}|${http.reason(451)}"
`
	const want = "OK|Non-Authoritative Information|Partial Content|Range Not Satisfiable|Unavailable For Legal Reasons\n"
	if got := runGeneratedVM(t, src, time.Minute); got != want {
		t.Fatalf("vm: got %q, want %q", got, want)
	}
	if got := runGeneratedAOT(t, src, time.Minute); got != want {
		t.Fatalf("native: got %q, want %q", got, want)
	}
}

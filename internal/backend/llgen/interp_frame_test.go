package llgen_test

import (
	"strings"
	"testing"
)

const interpFrameSrc = `
type Tag = On(n: Int) | Off

fn show(t: Tag) -> String {
    match t {
        On(n) -> "on ${n}"
        Off -> "off"
    }
}

fn fshow(v: Float?) -> String {
    match v {
        Some(f) -> "f ${f}"
        None -> "none"
    }
}

fn main() -> Void! {}
`

// A payload read is raw, so interpolation formats it from a frame immortal instead of boxing it.
func TestInterpolationFormatsARawScalarFromTheFrame(t *testing.T) {
	ir := emitIR(t, interpFrameSrc)
	for _, name := range []string{"show", "fshow"} {
		body := bodyOf(t, ir, `define ptr @"main.`+name+`"(`)
		if strings.Contains(body, "@rt_int(") || strings.Contains(body, "@rt_float(") || !strings.Contains(body, "@rt_const_num(") {
			t.Errorf("%s boxes the payload it interpolates:\n%s", name, body)
		}
	}
}

package llgen_test

import (
	"strings"
	"testing"
)

const toFloatSrc = `
fn scaled(xs: &Array[Int]) -> Array[Float] { xs.map((x: &Int) => x.toFloat() * 1.5) }

fn widened(xs: &Array[u8]) -> Array[Float] { xs.map((x: &u8) => x.toFloat()) }

fn narrowed(xs: &Array[Int]) -> Array[u8?] { xs.map((x: &Int) => x.toU8()) }

fn main() -> Void! {
    print "${scaled(&Array.of(3)).len()} ${widened(&Array.of(200)).len()} ${narrowed(&Array.of(300)).len()}"
}
`

func TestToFloatInACallbackComputesOnRegisters(t *testing.T) {
	ir := emitIR(t, toFloatSrc)
	for name, want := range map[string]string{
		"main.scaled.closure1":  "sitofp i64",
		"main.widened.closure1": "uitofp i64",
	} {
		body := fnBody(t, ir, name)
		if !strings.Contains(body, want) {
			t.Errorf("%s lacks %s:\n%s", name, want, body)
		}
		if strings.Contains(body, "@rt_convert(") {
			t.Errorf("%s boxes a conversion result it only computes with:\n%s", name, body)
		}
	}
	if body := fnBody(t, ir, "main.narrowed.closure1"); !strings.Contains(body, "@rt_convert(") {
		t.Errorf("a checked conversion still goes through the runtime:\n%s", body)
	}
}

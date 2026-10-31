package sem_test

import (
	"strings"
	"testing"
)

func TestInterruptHandlerUnwrapMayPanic(t *testing.T) {
	for name, body := range map[string]string{
		"unwrap": "o.unwrap()",
		"expect": `o.expect("present")`,
	} {
		t.Run(name, func(t *testing.T) {
			res := checkIsr(t, `
fn pick(o: Option[Int]) -> Int {
    `+body+`
}

export(interrupt) {
    pub fn tick() {
        let o: Option[Int] = Some(1)
        _ = pick(o)
    }
}
`)
			out := res.Render()
			if !res.HasErrors() || !strings.Contains(out, "E1081") || !strings.Contains(out, "may panic") {
				t.Fatalf("a handler reaching %s must be rejected as may-panic:\n%s", name, out)
			}
			if strings.Contains(out, "E2123") {
				t.Fatalf("%s must not be reported as a direct panic call:\n%s", name, out)
			}
		})
	}
}

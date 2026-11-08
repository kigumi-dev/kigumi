package driver_test

import (
	"regexp"
	"testing"
)

var entropySourceRows = []struct {
	triple string
	calls  []string
}{
	{"x86_64-linux-musl", []string{"getrandom"}},
	{"aarch64-macos", []string{"getentropy"}},
	{"x86_64-windows-gnu", []string{"BCryptGenRandom"}},
	{"wasm32-wasi", []string{"random_get"}},
	{"x86_64-freebsd", []string{"getentropy", "arc4random_buf"}},
	{"x86_64-netbsd", []string{"getentropy", "arc4random_buf"}},
	{"x86_64-openbsd", []string{"getentropy", "arc4random_buf"}},
}

func TestEntropySourceOnEachHostedOS(t *testing.T) {
	t.Parallel()
	const src = "fn main() -> Void! {\n    let b = host.entropy().bytes(300)?\n    print \"${b.len()}\"\n}\n"
	for _, row := range entropySourceRows {
		t.Run(row.triple, func(t *testing.T) {
			t.Parallel()
			ir := irFor(t, mustTarget(t, row.triple, ""), src)
			for _, name := range row.calls {
				if !regexp.MustCompile(`call [^@\n]*@` + name + `\(`).MatchString(ir) {
					t.Errorf("std/entropy on %s does not call %s", row.triple, name)
				}
			}
		})
	}
}

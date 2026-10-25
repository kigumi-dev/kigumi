package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/llgen"
)

const narrowCSrc = `
extern(C) {
    fn take(a: i8, b: u16, c: i32) -> u8
    fn word(x: u32) -> i32
}

export(C) {
    pub fn k_i8(x: i8) -> i8 { x.wrappingSub(1) }
    pub fn k_u8(x: u8) -> u8 { x.wrappingAdd(1) }
    pub fn k_u16(x: u16) -> u16 { x.wrappingAdd(1) }
    pub fn k_u32(x: u32) -> u32 { x }
}

fn main() {
    let r = unsafe { take(-1, 65535, 7) }
    let w = unsafe { word(3) }
    print "${r} ${w}"
}
`

// Extension attributes follow clang per target; export parameters carry none, so the callee re-extends them itself.
func TestNarrowCIntegersCarryExtensionAttributes(t *testing.T) {
	prog := plannedProgram(t, narrowCSrc)
	cases := []struct {
		arch, os string
		want     []string
	}{
		{"amd64", "linux", []string{
			"define signext i8 @k_i8(i8 %a0)", "define zeroext i8 @k_u8(i8 %a0)", "define zeroext i16 @k_u16(i16 %a0)", "define i32 @k_u32(i32 %a0)",
			"declare zeroext i8 @take(i8 signext, i16 zeroext, i32)", "declare i32 @word(i32)",
			"call zeroext i8 (i8, i16, i32) @take(i8 signext ", "call i32 (i32) @word(i32 ",
		}},
		{"riscv64", "linux", []string{
			"define signext i8 @k_i8(i8 %a0)", "define zeroext i16 @k_u16(i16 %a0)", "define signext i32 @k_u32(i32 %a0)",
			"declare zeroext i8 @take(i8 signext, i16 zeroext, i32 signext)", "declare signext i32 @word(i32 signext)",
			"call signext i32 (i32) @word(i32 signext ",
		}},
		{"arm64", "darwin", []string{
			"define signext i8 @k_i8(i8 %a0)", "declare zeroext i8 @take(i8 signext, i16 zeroext, i32)", "call zeroext i8 (i8, i16, i32) @take(i8 signext ",
		}},
		{"amd64", "windows", []string{
			"define i8 @k_i8(i8 %a0)", "declare i8 @take(i8, i16, i32)", "call i8 (i8, i16, i32) @take(i8 ",
		}},
		{"arm64", "linux", []string{
			"define i8 @k_i8(i8 %a0)", "define i16 @k_u16(i16 %a0)", "declare i8 @take(i8, i16, i32)", "call i8 (i8, i16, i32) @take(i8 ",
		}},
	}
	for _, c := range cases {
		ir := llgen.EmitWith(prog, llgen.EmitOptions{Arch: c.arch, OS: c.os})
		for _, want := range c.want {
			if !strings.Contains(ir, want) {
				t.Errorf("%s-%s: missing %q in IR", c.arch, c.os, want)
			}
		}
		if body := bodyOf(t, ir, "@k_i8(i8 %a0)"); !strings.Contains(body, "sext i8 %a0 to i64") {
			t.Errorf("%s-%s: the wrapper must extend its narrow parameter itself:\n%s", c.arch, c.os, body)
		}
	}
}

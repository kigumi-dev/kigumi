package sem

import (
	"fmt"
	"strconv"
	"strings"
)

var asmPrefixes = map[string]bool{"lock": true, "rep": true, "repe": true, "repz": true, "repne": true, "repnz": true}

// asmWrites names the instructions whose first operand is the only register they write; asmReads write none.
var (
	asmWrites = map[string]bool{
		"mov": true, "movzx": true, "movsx": true, "movsxd": true, "lea": true, "add": true, "sub": true,
		"adc": true, "sbb": true, "and": true, "or": true, "xor": true, "imul": true, "shl": true, "sal": true,
		"shr": true, "sar": true, "rol": true, "ror": true, "not": true, "neg": true, "inc": true, "dec": true,
		"bswap": true, "popcnt": true, "lzcnt": true, "tzcnt": true,
	}
	asmReads = map[string]bool{"cmp": true, "test": true, "bt": true, "nop": true, "cld": true, "pause": true}
)

// asmCalleeSaved are the SysV registers a function must hand back unchanged.
var asmCalleeSaved = map[string]bool{"rbx": true, "rbp": true, "r12": true, "r13": true, "r14": true, "r15": true}

var asmGPRs = func() map[string]string {
	m := map[string]string{}
	for _, r := range [][5]string{{"rax", "eax", "ax", "al", "ah"}, {"rcx", "ecx", "cx", "cl", "ch"}, {"rdx", "edx", "dx", "dl", "dh"}, {"rbx", "ebx", "bx", "bl", "bh"}, {"rsi", "esi", "si", "sil", ""}, {"rdi", "edi", "di", "dil", ""}, {"rbp", "ebp", "bp", "bpl", ""}, {"rsp", "esp", "sp", "spl", ""}} {
		for _, n := range r {
			if n != "" {
				m[n] = r[0]
			}
		}
	}
	for i := 8; i <= 15; i++ {
		q := fmt.Sprintf("r%d", i)
		for _, s := range []string{"", "d", "w", "b"} {
			m[q+s] = q
		}
	}
	return m
}()

// asmGPR is the 64-bit register an operand names, "" when it names none.
func asmGPR(op string) string { return asmGPRs[op] }

func asmImmediate(op string) (int64, bool) {
	n, err := strconv.ParseInt(op, 0, 64)
	return n, err == nil
}

func (v *asmVerifier) compute(ins, mnem string, ops []string) string {
	cmov, set := strings.HasPrefix(mnem, "cmov"), strings.HasPrefix(mnem, "set")
	switch {
	case mnem == "xchg":
		if len(ops) != 2 {
			return "`" + ins + "` cannot be classified"
		}
		for _, o := range ops {
			if why := v.writes(ins, o); why != "" {
				return why
			}
		}
		return ""
	case asmReads[mnem]:
		return v.operands(ins, ops)
	case !asmWrites[mnem] && !cmov && !set:
		return "`" + ins + "` cannot be classified as ordinary computation"
	case mnem == "imul" && len(ops) < 2:
		return "`" + ins + "` writes rdx:rax implicitly; use the two- or three-operand form"
	case len(ops) == 0:
		return "`" + ins + "` cannot be classified"
	}
	if why := v.operands(ins, ops[1:]); why != "" {
		return why
	}
	if asmGPR(ops[0]) == "rsp" {
		return v.adjustStack(ins, mnem, ops)
	}
	return v.writes(ins, ops[0])
}

// operands accepts registers, immediates, operand placeholders and lea's address forms as sources.
func (v *asmVerifier) operands(ins string, ops []string) string {
	for _, o := range ops {
		if _, ok := asmImmediate(o); ok || o == "%in" || o == "%out" || asmGPR(o) != "" || strings.Contains(o, "[") {
			continue
		}
		return "`" + ins + "` reads `" + o + "`, which cannot be classified"
	}
	return ""
}

func (v *asmVerifier) writes(ins, dst string) string {
	if dst == "%out" {
		return ""
	}
	reg := asmGPR(dst)
	switch {
	case dst == "%in":
		return "`" + ins + "` writes an input-only operand"
	case reg == "":
		return "`" + ins + "` writes `" + dst + "`, which cannot be classified"
	case reg == "rsp":
		return "`" + ins + "` moves the stack pointer in a way that cannot be verified"
	case !v.naked:
		return v.declared(ins, reg)
	case asmCalleeSaved[reg] && !v.holds(reg):
		return "`" + ins + "` overwrites callee-saved `" + reg + "` without pushing it first"
	}
	return ""
}

// declared requires a non-naked block to name every register it writes as a clobber or an explicit operand.
func (v *asmVerifier) declared(ins, reg string) string {
	for _, c := range v.info.Clobbers {
		if asmGPR(c) == reg {
			return ""
		}
	}
	for _, op := range v.info.Operands {
		if !op.Class && asmGPR(op.Reg) == reg && op.Kind != AsmIn {
			return ""
		}
	}
	return "`" + ins + "` writes `" + reg + "`, which the block neither clobbers nor outputs"
}

func (v *asmVerifier) holds(reg string) bool {
	for _, s := range v.saved {
		if s == reg {
			return true
		}
	}
	return false
}

// adjustStack accepts `sub rsp, N` and the matching `add rsp, N` that undoes it, in a naked body only.
func (v *asmVerifier) adjustStack(ins, mnem string, ops []string) string {
	n, ok := int64(0), false
	if len(ops) == 2 {
		n, ok = asmImmediate(ops[1])
	}
	if !v.naked || !ok || n <= 0 || n%8 != 0 || (mnem != "sub" && mnem != "add") {
		return "`" + ins + "` moves the stack pointer in a way that cannot be verified"
	}
	frame := "frame " + strconv.FormatInt(n, 10)
	if mnem == "sub" {
		v.saved = append(v.saved, frame)
		v.depth += n
		return ""
	}
	if k := len(v.saved); k == 0 || v.saved[k-1] != frame {
		return "`" + ins + "` does not undo the innermost `sub rsp`"
	}
	v.saved = v.saved[:len(v.saved)-1]
	v.depth -= n
	return ""
}

package sem

import (
	"strconv"
	"strings"
)

// asmNaked marks an EdgeAsm (in Why) recorded for a naked function's body.
const asmNaked = 1

// isrAsmVerdict is "" when the asm block behind e is verified against the amd64 SysV rules, else why not.
func (r *Result) isrAsmVerdict(e EffectEdge) string {
	info := r.AsmOf(e.File, e.Node)
	if info == nil {
		return "the asm block was not checked"
	}
	os := ""
	if r.mod != nil {
		os = r.mod.OS
	}
	return verifyIsrAsm(info, os, e.Why == asmNaked)
}

func verifyIsrAsm(info *AsmInfo, os string, naked bool) string {
	switch {
	case info.Arch != "amd64":
		return "asm for " + info.Arch + " cannot be verified yet; only amd64 has encoded ABI rules"
	case os == "windows" || os == "uefi":
		return "asm under the Microsoft x64 ABI cannot be verified yet"
	case info.Options["att_syntax"]:
		return "an AT&T syntax template cannot be verified; write it in Intel syntax"
	}
	v := &asmVerifier{info: info, naked: naked}
	for _, line := range info.Template {
		if why := asmRawProblem(line); why != "" {
			return why
		}
		for _, ins := range strings.FieldsFunc(v.expand(line), func(c rune) bool { return c == ';' || c == '\n' }) {
			if why := v.step(ins); why != "" {
				return why
			}
		}
	}
	if naked && !v.done {
		return "the naked body must end in `ret` or a `jmp` to a `sym` operand"
	}
	return ""
}

type asmVerifier struct {
	info  *AsmInfo
	naked bool
	// saved holds what each push or `sub rsp` put on the stack, innermost last; depth is its size in bytes.
	saved []string
	depth int64
	done  bool
}

// expand replaces placeholders: a const by its value, a sym by `@sym`, a register operand by `%in` or `%out`.
func (v *asmVerifier) expand(line string) string {
	pieces, _ := AsmPieces(line)
	var sb strings.Builder
	for _, p := range pieces {
		if p.Operand == -1 {
			sb.WriteString(p.Text)
			continue
		}
		op := v.info.Operands[v.info.Label(p.Text)]
		switch op.Kind {
		case AsmConst:
			sb.WriteString(strconv.FormatInt(op.Const, 10))
		case AsmSym:
			sb.WriteString("@sym")
		case AsmIn:
			sb.WriteString("%in")
		default:
			sb.WriteString("%out")
		}
	}
	return sb.String()
}

func (v *asmVerifier) step(ins string) string {
	if i := strings.IndexByte(ins, '#'); i >= 0 {
		ins = ins[:i]
	}
	ins = strings.ToLower(strings.TrimSpace(ins))
	if ins == "" {
		return ""
	}
	if v.done {
		return "`" + ins + "` follows the body's final `ret` or `jmp`"
	}
	mnem, rest, _ := strings.Cut(ins, " ")
	ops := splitAsmOperands(rest)
	switch {
	case strings.HasSuffix(mnem, ":") || strings.HasPrefix(mnem, "."):
		return "`" + ins + "` is a label or directive, which cannot be verified"
	case asmPrefixes[mnem]:
		return "`" + ins + "` carries a lock or repeat prefix, an unpermitted synchronization or memory operation"
	case strings.Contains(rest, "@sym") && mnem != "call" && mnem != "jmp":
		return "`" + ins + "` uses a symbol's address; only `call` or `jmp` may name a `sym` operand"
	case strings.ContainsAny(rest, "[") && mnem != "lea":
		return "`" + ins + "` accesses memory, which may be MMIO no configuration names"
	}
	switch mnem {
	case "call", "jmp", "ret":
		return v.control(mnem, ops)
	case "push", "pop":
		return v.stack(mnem, ops)
	}
	return v.compute(ins, mnem, ops)
}

func (v *asmVerifier) control(mnem string, ops []string) string {
	if !v.naked {
		return "inline asm may not `" + mnem + "`; only a naked handler controls its own frame"
	}
	switch mnem {
	case "call":
		if len(ops) != 1 || ops[0] != "@sym" {
			return "`call` reaches a target that is not a checked `sym` operand"
		}
		if (v.depth+8)%16 != 0 {
			return "`call` runs with the stack misaligned for the SysV ABI"
		}
		return ""
	case "jmp":
		if len(ops) != 1 || ops[0] != "@sym" {
			return "`jmp` reaches a target that is not a checked `sym` operand"
		}
	case "ret":
		if len(ops) != 0 {
			return "`ret` with an operand pops arguments the C ABI leaves to the caller"
		}
	}
	if len(v.saved) != 0 {
		return "`" + mnem + "` leaves the stack unbalanced; restore every push and `sub rsp` first"
	}
	v.done = true
	return ""
}

func (v *asmVerifier) stack(mnem string, ops []string) string {
	if !v.naked {
		return "inline asm may not `" + mnem + "`; the compiler owns the stack around it"
	}
	if len(ops) != 1 || asmGPR(ops[0]) != ops[0] || ops[0] == "rsp" {
		return "`" + mnem + "` takes one 64-bit general register here"
	}
	if mnem == "push" {
		v.saved = append(v.saved, ops[0])
		v.depth += 8
		return ""
	}
	if n := len(v.saved); n == 0 || v.saved[n-1] != ops[0] {
		return "`pop " + ops[0] + "` does not restore the register pushed last"
	}
	v.saved = v.saved[:len(v.saved)-1]
	v.depth -= 8
	return ""
}

// asmRawProblem rejects text the verifier and the assembler could split differently: `\r` ends a `#` comment
// and `/*` opens one for the assembler only, and a literal `@` reads like the verifier's own sym placeholder.
func asmRawProblem(line string) string {
	pieces, _ := AsmPieces(line)
	for _, p := range pieces {
		if p.Operand != -1 {
			continue
		}
		for _, c := range p.Text {
			switch {
			case c == '@':
				return "the template writes `@`, which only a `sym` operand may produce"
			case c == '/':
				return "the template writes `/`, which may open a comment the verifier cannot follow"
			case c < 0x20 && c != '\t' && c != '\n' || c == 0x7f:
				return "the template contains the control character " + strconv.QuoteRune(c) + ", which may hide an instruction from the verifier"
			}
		}
	}
	return ""
}

func splitAsmOperands(rest string) []string {
	var out []string
	for _, o := range strings.Split(rest, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

type llSig struct {
	ret      string
	params   []string
	variadic bool
}

const boolAggSrc = `
type Flag = {
    pub live Bool
    pub n Int
}

fn mk(n: Int) -> Flag { Flag { live: n > 0, n: n } }

fn stash(n: Int) -> Array[Flag] { Array.of(mk(n), mk(0 - n)) }

fn firstLive(fs: &Array[Flag]) -> Bool {
    if fs.get(0) is Some(f) { f.live } else { false }
}

fn main() -> Void! {}
`

// Keeps a quoted type name whole: %"kg.agg.Tuple2[Int,Int]" is one token with a
// space in it.
func cutLLToken(s string) (string, string) {
	s = strings.TrimLeft(s, " ")
	quoted := false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '"':
			quoted = !quoted
		case s[i] == ' ' && !quoted:
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func splitLLList(s string) []string {
	var out []string
	depth, quoted, start := 0, false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func balanced(s string) (string, string, bool) {
	depth, quoted := 0, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:], true
			}
		}
	}
	return "", "", false
}

func calleeAndArgs(s string) (string, string, bool) {
	s = strings.TrimLeft(s, " ")
	if s == "" || s[0] != '@' {
		return "", "", false
	}
	i := 1
	if i < len(s) && s[i] == '"' {
		for i++; i < len(s) && s[i] != '"'; i++ {
		}
		i++
	} else {
		for i < len(s) && s[i] != '(' {
			i++
		}
	}
	if i >= len(s) || s[i] != '(' {
		return "", "", false
	}
	inside, _, ok := balanced(s[i:])
	if !ok {
		return "", "", false
	}
	return s[:i], inside, true
}

func llTypes(list string) []string {
	parts := splitLLList(list)
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i], _ = cutLLToken(p)
	}
	return out
}

func moduleSigs(ir string) map[string]llSig {
	out := map[string]llSig{}
	for _, line := range strings.Split(ir, "\n") {
		rest := ""
		switch {
		case strings.HasPrefix(line, "declare "):
			rest = line[len("declare "):]
		case strings.HasPrefix(line, "define "):
			rest = strings.TrimSuffix(strings.TrimSpace(line[len("define "):]), "{")
		default:
			continue
		}
		for _, kw := range []string{"internal ", "private ", "external ", "dso_local "} {
			rest = strings.TrimPrefix(rest, kw)
		}
		ret, tail := cutLLToken(rest)
		name, args, ok := calleeAndArgs(tail)
		if !ok {
			continue
		}
		sig := llSig{ret: ret, params: llTypes(args)}
		if n := len(sig.params); n > 0 && sig.params[n-1] == "..." {
			sig.params, sig.variadic = sig.params[:n-1], true
		}
		out[name] = sig
	}
	return out
}

// Skips the explicit function type an FFI call carries.
func parseLLCall(line string) (string, string, []string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(line, " ") {
		return "", "", nil, false
	}
	if i := strings.Index(s, " = "); i >= 0 && strings.HasPrefix(s, "%") {
		s = s[i+3:]
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "musttail "), "tail ")
	if !strings.HasPrefix(s, "call ") {
		return "", "", nil, false
	}
	ret, tail := cutLLToken(s[len("call "):])
	if tail = strings.TrimLeft(tail, " "); strings.HasPrefix(tail, "(") {
		if _, rest, ok := balanced(tail); ok {
			tail = rest
		}
	}
	name, args, ok := calleeAndArgs(tail)
	if !ok {
		return "", "", nil, false
	}
	return ret, name, llTypes(args), true
}

// A call site stating a type its callee does not take is how an i1 reaches
// a runtime parameter the C side writes as `int`.
func TestEmittedCallsMatchDeclarations(t *testing.T) {
	for _, src := range []string{boolAggSrc, directAggSrc} {
		ir := emitIR(t, src)
		sigs := moduleSigs(ir)
		if len(sigs) < 50 {
			t.Fatalf("only %d signatures parsed out of the module; the reader drifted from the emitter", len(sigs))
		}
		checked := 0
		for _, line := range strings.Split(ir, "\n") {
			ret, name, args, ok := parseLLCall(line)
			if !ok {
				continue
			}
			sig, known := sigs[name]
			if !known {
				continue
			}
			checked++
			if len(args) != len(sig.params) && !(sig.variadic && len(args) > len(sig.params)) {
				t.Errorf("%s takes %v, the call passes %v:\n%s", name, sig.params, args, line)
				continue
			}
			for i, want := range sig.params {
				if args[i] != want {
					t.Errorf("%s parameter %d is %s, the call passes %s:\n%s", name, i, want, args[i], line)
				}
			}
			if ret != sig.ret {
				t.Errorf("%s returns %s, the call reads %s:\n%s", name, sig.ret, ret, line)
			}
		}
		if checked < 100 {
			t.Fatalf("only %d call sites checked; the reader drifted from the emitter", checked)
		}
	}
}

// Boxing a direct aggregate widens its Bool fields, the way the scalar
// path does: rt_bool is written as taking an `int`.
func TestDirectAggregateBoxWidensBool(t *testing.T) {
	ir := emitIR(t, boolAggSrc)
	if strings.Contains(ir, "@rt_bool(i1 ") {
		t.Error("a Bool reaches rt_bool as an i1")
	}
	body := bodyOf(t, ir, `define internal ptr @"kg.box.main.Flag"(`)
	if !regexp.MustCompile(`zext i1 %v\d+ to i32`).MatchString(body) || !strings.Contains(body, "@rt_bool(i32 ") {
		t.Errorf("the boxing helper does not widen its Bool field:\n%s", body)
	}
}

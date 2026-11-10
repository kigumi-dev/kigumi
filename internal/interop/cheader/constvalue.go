package cheader

import "strings"

func classifyConst(name, typ, literal string, abi ABI, renames map[string]string) item {
	kt, ok := mapScalar(typ, abi)
	if !ok {
		return item{name: name, skip: "unsupported constant type `" + typ + "`"}
	}
	out := emittedName(name, renames)
	return item{kind: "const", name: out, text: "const " + out + ": " + kt + " = " + literal, scalar: kt, lit: literal}
}

// classifyBareConst disambiguates `pub const NAME = TARGET;`, which zig uses
// for a plain alias, a function-pointer typedef, and a macro constant alike.
func classifyBareConst(chunk string, known map[string]string, opaque map[string]bool, vals map[string]cval, abi ABI, renames map[string]string) item {
	m := reConstBare.FindStringSubmatch(chunk)
	name, target := m[1], strings.TrimSpace(m[2])
	switch {
	case strings.HasPrefix(target, "__helpers.cast(?*") || strings.HasPrefix(target, "__helpers.cast(*"):
		return item{name: name, skip: "pointer-valued macro (Kigumi has no pointer constant to spell it with)"}
	case isIdent(target):
		if v, ok := vals[target]; ok {
			vals[name] = v
			out := emittedName(name, renames)
			return item{kind: "const", name: out, text: "const " + out + ": " + v.typeName() + " = " + v.v.String()}
		}
		return classifyAlias(name, target, known, opaque, abi, renames)
	case looksLikeType(target):
		// A function-pointer typedef (`Callback = ?*const fn (...) ...`):
		// zig's `pub const` covers both values and types, so a pointer or
		// `fn` prefix is the only signal this is the latter.
		kt, ok := mapType(target, known, opaque, abi)
		if !ok {
			return item{name: name, skip: "unsupported function pointer type"}
		}
		friendly := stripTagPrefix(name)
		out := emittedName(friendly, renames)
		known[friendly] = out
		return item{kind: "alias", name: out, text: "type " + out + " = alias " + kt}
	}
	typ, lit, v, ok := evalConstEnv(target, abi, vals)
	if !ok {
		return item{name: name, skip: "unsupported constant value"}
	}
	if v.v != nil {
		vals[name] = v
	}
	out := emittedName(name, renames)
	return item{kind: "const", name: out, text: "const " + out + ": " + typ + " = " + lit}
}

func looksLikeType(s string) bool {
	for _, p := range []string{"?*", "[*c]", "*", "fn "} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// classifyAlias handles `pub const NAME = TARGET;` where TARGET is a bare
// identifier: either a scalar keyword (an imported enum's underlying type
// becomes a named alias to it) or another name this pass already emitted.
// The classic C `typedef struct X { ... } X;` shape reaches here as its own
// redundant alias chunk (X = struct_X) once the struct itself is emitted
// under the stripped name X; that case is silently coalesced away.
func classifyAlias(name, target string, known map[string]string, opaque map[string]bool, abi ABI, renames map[string]string) item {
	if kt, ok := mapScalar(target, abi); ok {
		friendly := stripTagPrefix(name)
		out := emittedName(friendly, renames)
		known[friendly] = out
		return item{kind: "alias", name: out, text: "type " + out + " = alias " + kt, scalar: kt}
	}
	resolved := target
	emitted, ok := known[resolved]
	if !ok {
		if s := stripTagPrefix(target); s != resolved {
			resolved, emitted = s, known[s]
		}
		if emitted == "" {
			return item{name: name, skip: "alias to unresolved `" + target + "`"}
		}
	}
	friendly := stripTagPrefix(name)
	if friendly == resolved {
		return item{kind: "noop"}
	}
	out := emittedName(friendly, renames)
	known[friendly] = out
	// An alias to an opaque name is itself pointer-only: `typedef struct
	// Bitfield Bitfield2;` must reject a by-value `Bitfield2` just like
	// `Bitfield`.
	if opaque[resolved] {
		opaque[friendly] = true
	}
	return item{kind: "alias", name: out, text: "type " + out + " = alias " + emitted}
}

package cheader

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var reRecord = regexp.MustCompile(`(?s)^pub const (\w+) = extern (struct|union) \{(.*)\}$`)
var reOpaqueName = regexp.MustCompile(`^pub const (\w+) = opaque \{`)
var reExternStructHead = regexp.MustCompile(`^pub const (\w+) = extern struct \{`)

// fieldAlign records one record field's explicit C alignment overrides.
type fieldAlign struct {
	n     int
	first bool // the field sits at offset 0, so aligning it cannot move it
}

// cutFieldAlign splits translate-c's ` align(N)` field suffix off a type
// expression: it is a C field/record alignment override, not part of the type.
func cutFieldAlign(s string) (typ string, align int, ok bool) {
	if !strings.HasSuffix(s, ")") {
		return s, 0, false
	}
	i := strings.LastIndex(s, " align(")
	if i < 0 {
		return s, 0, false
	}
	n, err := strconv.Atoi(s[i+len(" align(") : len(s)-1])
	if err != nil || n <= 0 {
		return s, 0, false
	}
	return strings.TrimSpace(s[:i]), n, true
}

// recordLayout maps translate-c's per-field align(N) annotations to the one
// layout clause they stand for, or ok=false when the C layout is not one
// Kigumi can express. Kigumi has `layout(C, packed)` (every field align 1)
// and `layout(C, align: N)` (raise the record's alignment), but no per-field
// alignment override, and `#pragma pack(N)` for N>1 lowers field alignment
// instead of raising the record's, so those shapes stay E994.
func recordLayout(aligns []fieldAlign) (clause string, ok bool) {
	if len(aligns) == 0 {
		return "", true
	}
	packed := true
	for _, a := range aligns {
		if a.n != 1 {
			packed = false
			break
		}
	}
	if packed {
		return ", packed", true
	}
	// A single annotated field that is also the first one only changes the
	// record's alignment (the field stays at offset 0), which is what
	// `layout(C, align: N)` reproduces; any other field would need its own
	// rounded-up offset.
	if len(aligns) == 1 && aligns[0].first && isPow2(int64(aligns[0].n)) && aligns[0].n <= 16 {
		return fmt.Sprintf(", align: %d", aligns[0].n), true
	}
	return "", false
}

func isPow2(n int64) bool { return n > 0 && n&(n-1) == 0 }

// classifyRecord emits `type X layout(C...) = { fields }`. known maps the
// header's C type spellings to Kigumi names; a record's own name is
// pre-registered before its fields are walked, so self-referential and
// mutually recursive structs resolve.
func classifyRecord(chunk string, known map[string]string, opaque map[string]bool, abi ABI, renames map[string]string) item {
	m := reRecord.FindStringSubmatch(chunk)
	name, kind, body := m[1], m[2], strings.TrimSpace(m[3])
	if kind == "union" {
		return item{name: stripTagPrefix(name), skip: "C union (Kigumi has no ABI-safe union type)"}
	}
	body = dropNamespaceMembers(body)
	if body == "" {
		return registerOpaque(name, known, opaque, renames)
	}
	var fields []string
	var aligns []fieldAlign
	for _, raw := range splitTopCommas(body) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		fname, rest, ok := splitParam(raw)
		if !ok {
			return item{name: emittedName(stripTagPrefix(name), renames), skip: "unsupported field in `" + stripTagPrefix(name) + "`"}
		}
		ftyp, _, _ := strings.Cut(rest, "=")
		ftyp = strings.TrimSpace(ftyp)
		if at, align, hasAlign := cutFieldAlign(ftyp); hasAlign {
			ftyp = at
			aligns = append(aligns, fieldAlign{n: align, first: len(fields) == 0})
		}
		if bareOpaqueByValue(ftyp, opaque) {
			return item{name: emittedName(stripTagPrefix(name), renames), skip: "field `" + fname + "` of `" + stripTagPrefix(name) + "` holds `" + stripTagPrefix(ftyp) + "` by value (opaque type; zig demoted it, most likely for a bitfield, so only a pointer to it crosses the C ABI)"}
		}
		kt, ok := mapByValueType(ftyp, known, opaque, abi)
		if !ok && abi.Int128 && (ftyp == "i128" || ftyp == "u128") {
			kt, ok = ftyp, true
		}
		if !ok {
			return item{name: emittedName(stripTagPrefix(name), renames), skip: "unsupported field `" + fname + "` in `" + stripTagPrefix(name) + "`"}
		}
		fields = append(fields, "    pub "+sanitizeIdent(fname)+" "+kt)
	}
	friendly := stripTagPrefix(name)
	out := emittedName(friendly, renames)
	clause, layoutOK := recordLayout(aligns)
	if !layoutOK {
		return item{name: out, skip: "`" + friendly + "`'s field alignment overrides (pack(N) or a member alignment) are not expressible as a Kigumi layout"}
	}
	known[friendly] = out
	text := "type " + out + " layout(C" + clause + ") = {\n" + strings.Join(fields, "\n") + "\n}"
	return item{kind: "record", name: out, text: text}
}

// dropNamespaceMembers removes the `pub const f = __root.f;` lines zig adds
// inside a struct body for a function whose first parameter is that struct
// (a UFCS-style convenience Kigumi has no use for); a real field's line
// never starts with `pub`, so this is enough to tell the two apart.
func dropNamespaceMembers(body string) string {
	var kept []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "pub ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// widenEnumMembers retypes a run of enum member consts (pending, indices
// into consts) to scalar: zig gives each member its own type independently
// of the enum's, so a run of `c_int` members can sit right before an
// `enum_X = c_uint` alias (every member non-negative) and disagree with it.
func widenEnumMembers(consts []item, pending []int, scalar string) {
	for _, i := range pending {
		if consts[i].scalar == scalar {
			continue
		}
		consts[i].scalar = scalar
		consts[i].text = "const " + consts[i].name + ": " + scalar + " = " + consts[i].lit
	}
}

// registerOpaque records a type zig kept bodiless (`pub const X = opaque {`,
// or a struct body with nothing left after namespace members were dropped):
// only a pointer to it is ABI-safe.
func registerOpaque(name string, known map[string]string, opaque map[string]bool, renames map[string]string) item {
	friendly := stripTagPrefix(name)
	out := emittedName(friendly, renames)
	known[friendly] = out
	opaque[friendly] = true
	return item{kind: "opaque", name: out, text: "type " + out}
}

func classifyOpaque(chunk string, known map[string]string, opaque map[string]bool, renames map[string]string) item {
	m := reOpaqueName.FindStringSubmatch(chunk)
	return registerOpaque(m[1], known, opaque, renames)
}

// ownChunks cuts full into the header's own top-level declarations, dropping
// zig's built-in predefines (the ones the empty-baseline header also emits).
func ownChunks(full, baseline string) []string {
	seen := map[string]bool{}
	for _, c := range splitTopLevel(baseline) {
		if k, n, ok := declHead(c); ok {
			seen[k+" "+n] = true
		}
	}
	var chunks []string
	for _, c := range splitTopLevel(full) {
		if k, n, ok := declHead(c); ok && !seen[k+" "+n] {
			chunks = append(chunks, c)
		}
	}
	return chunks
}

// keywordRenames gives every declaration whose C name is a Kigumi keyword a
// free Kigumi name (`fn` -> `fn_`, and `match_` in the same header forces
// `match__`). Header order decides the suffixes, so the result is stable.
// Types, functions and macro constants all go through it: C allows
// `mut`/`type`/`fn` as identifiers, but Kigumi's generated package has to
// parse.
func keywordRenames(chunks []string, taken map[string]bool) map[string]string {
	renames := map[string]string{}
	for _, c := range chunks {
		_, n, ok := declHead(c)
		if !ok {
			continue
		}
		name := stripTagPrefix(unquoteIdent(n))
		if !kigumiKeywords[name] {
			continue
		}
		if _, done := renames[name]; done {
			continue
		}
		out := name + "_"
		for taken[out] {
			out += "_"
		}
		taken[out] = true
		renames[name] = out
	}
	return renames
}

// emittedName is a declaration's Kigumi name, renamed when its C name is a
// keyword.
func emittedName(name string, renames map[string]string) string {
	if out, ok := renames[name]; ok {
		return out
	}
	return name
}

func declaredNames(full string) map[string]bool {
	taken := map[string]bool{}
	for _, c := range splitTopLevel(full) {
		if _, n, ok := declHead(c); ok {
			taken[stripTagPrefix(unquoteIdent(n))] = true
		}
	}
	return taken
}

// preRegName is the declaration name of a chunk whose record body has to be
// resolvable while its own fields are walked (self/mutual recursion).
func preRegName(chunk string) (string, bool) {
	m := reExternStructHead.FindStringSubmatch(chunk)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// Generate turns a header's zig translate-c output (full) into Kigumi
// source, using the same tool's output on an empty header (baseline) to
// tell the header's own declarations from zig's built-in predefines.
// skipped lists what was left out, one entry per declaration, sorted and
// deduplicated by name.
func Generate(full, baseline string, abi ABI) (source string, skipped []string) {
	return GenerateWith(full, baseline, abi, nil)
}

func GenerateWith(full, baseline string, abi ABI, wide map[string]bool) (source string, skipped []string) {
	chunks := ownChunks(full, baseline)
	renames := keywordRenames(chunks, declaredNames(full))
	// Pre-register every record name so a struct's own fields (and a sibling
	// declared later) resolve. A record whose body still cannot be laid out
	// drops its name and the pass reruns, so references to it fail as they
	// did before instead of surviving as dangling type names.
	kept := map[string]bool{}
	for _, c := range chunks {
		if n, ok := preRegName(c); ok {
			kept[n] = true
		}
	}
	var all []item
	var skip []string
	for pass := 0; ; pass++ {
		known, opaque := map[string]string{}, map[string]bool{}
		for n := range kept {
			friendly := stripTagPrefix(n)
			known[friendly] = emittedName(friendly, renames)
		}
		var items []item
		items, skip = classifyChunks(chunks, known, opaque, abi, renames, wide)
		emitted := map[string]bool{}
		for _, it := range items {
			if it.kind == "record" || it.kind == "opaque" {
				emitted[it.name] = true
			}
		}
		all = items
		var dropped []string
		for n := range kept {
			if !emitted[emittedName(stripTagPrefix(n), renames)] {
				dropped = append(dropped, n)
			}
		}
		if len(dropped) == 0 || pass >= len(chunks) {
			break
		}
		for _, n := range dropped {
			delete(kept, n)
		}
	}
	var opaques, recs, aliases, fns, consts []item
	for _, it := range all {
		switch it.kind {
		case "opaque":
			opaques = append(opaques, it)
		case "record":
			recs = append(recs, it)
		case "alias":
			aliases = append(aliases, it)
		case "fn":
			fns = append(fns, it)
		case "const":
			consts = append(consts, it)
		}
	}
	byName := func(s []item) { sort.Slice(s, func(i, j int) bool { return s[i].name < s[j].name }) }
	byName(opaques)
	byName(recs)
	byName(aliases)
	byName(fns)
	byName(consts)
	if n := strings.Count(full, "demoted to opaque type"); n > 0 {
		skip = append(skip, fmt.Sprintf("%d struct(s) use bitfields or another feature Kigumi can't lay out; imported as opaque types (fields inaccessible)", n))
	}
	sort.Strings(skip)
	return render(opaques, recs, aliases, fns, consts), dedupe(skip)
}

// classifyChunks runs one classification pass over the header's own
// declarations, in header order, and returns every item plus the skips.
func classifyChunks(chunks []string, known map[string]string, opaque map[string]bool, abi ABI, renames map[string]string, wide map[string]bool) (items []item, skip []string) {
	vals := map[string]cval{}
	var pendingMembers []int
	for _, c := range chunks {
		k, n, _ := declHead(c)
		it := classify(c, known, opaque, vals, abi, renames)
		if k == "const" && wide[n] && it.skip == "" {
			it = item{name: n, skip: "wide string literal (L, u or U prefix); Kigumi has no wide string type"}
		}
		switch {
		case it.kind == "noop":
			continue
		case it.skip != "":
			skip = append(skip, it.name+": "+it.skip)
			pendingMembers = nil
			continue
		}
		items = append(items, it)
		if it.kind == "alias" && it.scalar != "" {
			widenEnumMembers(items, pendingMembers, it.scalar)
		}
		if it.kind == "const" && it.scalar != "" {
			pendingMembers = append(pendingMembers, len(items)-1)
		} else {
			pendingMembers = nil
		}
	}
	return items, skip
}

package llgen

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	opdef "kigumi/internal/compiler/mir/ops"
)

//go:generate env UPDATE=1 go test -run TestStdDispatchFresh .

//go:embed std_dispatch_bodies.txt
var stdDispatchBodies string

const stdDispatchHead = `val rt_std_id(int op, int n, val* a) {
    val a0 = n > 0 ? rt_deref(a[0]) : NULL, a1 = n > 1 ? rt_deref(a[1]) : NULL, a2 = n > 2 ? rt_deref(a[2]) : NULL;
    if (a0 && a0->kind == K_BOX && !op_stores_a0(op)) a0 = a0->u.box.v;
    switch ((enum std_op)op) {
    RT_STD_OPS_RETIRED_CASES
        rt_panic("retired std op");
        return NULL;
#ifdef KIGUMI_OP_SELECT
    default: break;
#endif
`

const stdDispatchTail = `    }
    rt_panic("rt_std_id: bad op");
    return NULL;
}
`

// stdDispatchKind is the directive of std_dispatch_bodies.txt an entry came from.
type stdDispatchKind int

const (
	// stdDispatchSection is a `    // text` comment between the case blocks.
	stdDispatchSection stdDispatchKind = iota
	// stdDispatchOp is one op's case, emitted under `#if defined(KIGUMI_OP_<NAME>)`.
	stdDispatchOp
	// stdDispatchCap is an `#ifdef KIGUMI_CAP_<...>` group whose absent-capability `#else` keeps a panic stub per op.
	stdDispatchCap
)

// stdDispatchBlock is one entry of std_dispatch_bodies.txt: a section comment, an op's case or a capability group.
type stdDispatchBlock struct {
	Kind  stdDispatchKind
	Text  string             // the comment text, or the capability a cap group's stubs name ("host.shell")
	Macro string             // the KIGUMI_CAP_ macro guarding a cap group
	Names []string           // the ops an op case answers; several names share one body
	Body  []string           // the C lines after the `case` labels, an inline body's first line keeping its leading space
	Arms  []stdDispatchBlock // a cap group's case arms
	Stubs [][]string         // a cap group's `#else` stubs, one per guard, listing the ops sharing one panic
}

var stdCapMacroRe = regexp.MustCompile(`^KIGUMI_CAP_[A-Z0-9_]+$`)

// StdDispatch renders runtime/rt_std_dispatch.inc from the op registry and the hand-written
// C bodies in std_dispatch_bodies.txt: for every live op one `case OP_<NAME>` under
// `#if defined(KIGUMI_OP_<NAME>)`, the capability groups with their absence stubs, the retired
// cases (RT_STD_OPS_RETIRED_CASES, which rt_std_ops.h expands) and the bad-op default panic.
// The table is keyed by exact op name and holds the only hand-written C: the generator checks it
// answers every live registry op exactly once, and no retired or unknown op.
// The directive lines of the table are `@section text`, `@op name`, `@cap macro capability`,
// `@arm name...`, `@stub name...`, `@endcap` and the ignored `@note text`; every other line is C
// belonging to the `@op`/`@arm` above it, and a single-name body's first line is appended to the
// `case OP_<NAME>:` label, so it starts with a space (or is empty).
func StdDispatch(reg *opdef.StdOpRegistry) (string, error) {
	blocks, err := parseStdDispatchBodies(stdDispatchBodies)
	if err != nil {
		return "", err
	}
	if err := checkStdDispatchBlocks(reg, blocks); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("// Generated from internal/compiler/mir/ops/stdops.reg and std_dispatch_bodies.txt by TestStdDispatchFresh; do not edit.\n")
	sb.WriteString(stdDispatchHead)
	for _, b := range blocks {
		writeStdDispatchBlock(&sb, b)
	}
	sb.WriteString(stdDispatchTail)
	return sb.String(), nil
}

func parseStdDispatchBodies(text string) ([]stdDispatchBlock, error) {
	var blocks []stdDispatchBlock
	var cap *stdDispatchBlock
	var open *stdDispatchBlock
	closeOpen := func() {
		if open == nil {
			return
		}
		if cap != nil {
			cap.Arms = append(cap.Arms, *open)
		} else {
			blocks = append(blocks, *open)
		}
		open = nil
	}
	for n, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("std_dispatch_bodies.txt:%d: %s", n+1, fmt.Sprintf(format, args...))
		}
		if !strings.HasPrefix(line, "@") {
			if open == nil {
				return nil, fail("C body line outside @op/@arm: %q", line)
			}
			open.Body = append(open.Body, line)
			continue
		}
		word, rest, _ := strings.Cut(line, " ")
		switch word {
		case "@note":
		case "@section":
			closeOpen()
			if cap != nil {
				return nil, fail("@section inside a cap group")
			}
			if rest == "" {
				return nil, fail("@section wants a comment text")
			}
			blocks = append(blocks, stdDispatchBlock{Kind: stdDispatchSection, Text: rest})
		case "@op":
			closeOpen()
			if cap != nil {
				return nil, fail("@op inside a cap group")
			}
			open = &stdDispatchBlock{Kind: stdDispatchOp, Names: []string{rest}}
		case "@cap":
			closeOpen()
			if cap != nil {
				return nil, fail("@cap inside a cap group")
			}
			macro, name, ok := strings.Cut(rest, " ")
			if !ok || name == "" {
				return nil, fail("@cap wants MACRO CAPABILITY")
			}
			cap = &stdDispatchBlock{Kind: stdDispatchCap, Macro: macro, Text: name}
		case "@arm":
			closeOpen()
			if cap == nil {
				return nil, fail("@arm outside a cap group")
			}
			open = &stdDispatchBlock{Kind: stdDispatchOp, Names: strings.Fields(rest)}
		case "@stub":
			closeOpen()
			if cap == nil {
				return nil, fail("@stub outside a cap group")
			}
			cap.Stubs = append(cap.Stubs, strings.Fields(rest))
		case "@endcap":
			closeOpen()
			if cap == nil {
				return nil, fail("@endcap without @cap")
			}
			blocks = append(blocks, *cap)
			cap = nil
		default:
			return nil, fail("unknown directive %q", word)
		}
	}
	closeOpen()
	if cap != nil {
		return nil, fmt.Errorf("std_dispatch_bodies.txt: @cap %s is never closed", cap.Macro)
	}
	return blocks, nil
}

// checkStdDispatchBlocks proves the table answers the registry: every live op exactly once, no retired or unknown op, and every capability arm stubbed for the builds without it.
func checkStdDispatchBlocks(reg *opdef.StdOpRegistry, blocks []stdDispatchBlock) error {
	live := make(map[string]bool, len(reg.Ops))
	for _, op := range reg.Ops {
		if !op.Retired {
			live[op.Name] = true
		}
	}
	seen := map[string]bool{}
	claim := func(name string) error {
		if seen[name] {
			return fmt.Errorf("std_dispatch_bodies.txt: op %s has two bodies", name)
		}
		if !live[name] {
			return fmt.Errorf("std_dispatch_bodies.txt: op %s is not a live registry op", name)
		}
		seen[name] = true
		return nil
	}
	claimArm := func(arm stdDispatchBlock, macro string) (map[string]bool, error) {
		if len(arm.Names) == 0 {
			if macro == "" {
				return nil, fmt.Errorf("std_dispatch_bodies.txt: @op wants an op name")
			}
			return nil, fmt.Errorf("std_dispatch_bodies.txt: cap %s has an arm with no op name", macro)
		}
		if len(arm.Body) == 0 {
			if macro == "" {
				return nil, fmt.Errorf("std_dispatch_bodies.txt: op %s has no body", arm.Names[0])
			}
			return nil, fmt.Errorf("std_dispatch_bodies.txt: cap %s has an arm with no body", macro)
		}
		if len(arm.Names) == 1 && arm.Body[0] != "" && !strings.HasPrefix(arm.Body[0], " ") {
			return nil, fmt.Errorf("std_dispatch_bodies.txt: op %s's body must start on the label line after a space", arm.Names[0])
		}
		names := map[string]bool{}
		for _, name := range arm.Names {
			if err := claim(name); err != nil {
				return nil, err
			}
			names[name] = true
		}
		return names, nil
	}
	for _, b := range blocks {
		switch b.Kind {
		case stdDispatchSection:
		case stdDispatchOp:
			if len(b.Names) != 1 {
				return fmt.Errorf("std_dispatch_bodies.txt: top-level @op wants one name, got %v", b.Names)
			}
			if _, err := claimArm(b, ""); err != nil {
				return err
			}
		case stdDispatchCap:
			if !stdCapMacroRe.MatchString(b.Macro) {
				return fmt.Errorf("std_dispatch_bodies.txt: cap macro %q is not a KIGUMI_CAP_ name", b.Macro)
			}
			if len(b.Arms) == 0 {
				return fmt.Errorf("std_dispatch_bodies.txt: cap %s has no arms", b.Macro)
			}
			if len(b.Stubs) == 0 {
				return fmt.Errorf("std_dispatch_bodies.txt: cap %s has no absence stubs", b.Macro)
			}
			arms := map[string]bool{}
			for _, arm := range b.Arms {
				names, err := claimArm(arm, b.Macro)
				if err != nil {
					return err
				}
				for name := range names {
					if arms[name] {
						return fmt.Errorf("std_dispatch_bodies.txt: cap %s answers op %s twice", b.Macro, name)
					}
					arms[name] = true
				}
			}
			stubbed := map[string]bool{}
			for _, stub := range b.Stubs {
				if len(stub) == 0 {
					return fmt.Errorf("std_dispatch_bodies.txt: cap %s has an empty stub", b.Macro)
				}
				for _, name := range stub {
					if stubbed[name] {
						return fmt.Errorf("std_dispatch_bodies.txt: cap %s stubs op %s twice", b.Macro, name)
					}
					if !arms[name] {
						return fmt.Errorf("std_dispatch_bodies.txt: cap %s stubs op %s, which none of its arms answers", b.Macro, name)
					}
					stubbed[name] = true
				}
			}
			for name := range arms {
				if !stubbed[name] {
					return fmt.Errorf("std_dispatch_bodies.txt: cap %s answers op %s but has no absence stub for it", b.Macro, name)
				}
			}
		}
	}
	var missing []string
	for _, op := range reg.Ops {
		if !op.Retired && !seen[op.Name] {
			missing = append(missing, op.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("std_dispatch_bodies.txt: no body for live op(s) %s", strings.Join(missing, ", "))
	}
	return nil
}

func writeStdDispatchBlock(sb *strings.Builder, b stdDispatchBlock) {
	switch b.Kind {
	case stdDispatchSection:
		fmt.Fprintf(sb, "    // %s\n", b.Text)
	case stdDispatchOp:
		writeStdArm(sb, b.Names, b.Body)
	case stdDispatchCap:
		fmt.Fprintf(sb, "#ifdef %s\n", b.Macro)
		for _, arm := range b.Arms {
			writeStdArm(sb, arm.Names, arm.Body)
		}
		sb.WriteString("#else\n")
		for _, stub := range b.Stubs {
			writeStdStub(sb, stub, b.Text)
		}
		sb.WriteString("#endif\n")
	}
}

// writeStdArm emits one guarded `case` arm; with several names the labels stand alone and share the body.
func writeStdArm(sb *strings.Builder, names, body []string) {
	sb.WriteString("#if ")
	for i, name := range names {
		if i > 0 {
			sb.WriteString(" || ")
		}
		fmt.Fprintf(sb, "defined(KIGUMI_OP_%s)", name)
	}
	sb.WriteString("\n")
	for _, name := range names {
		if len(names) == 1 {
			fmt.Fprintf(sb, "    case OP_%s:", name)
		} else {
			fmt.Fprintf(sb, "    case OP_%s:\n", name)
		}
	}
	for _, line := range body {
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	sb.WriteString("#endif\n")
}

// writeStdStub panics for every op of one guard when the build lacks the capability the case bodies need.
func writeStdStub(sb *strings.Builder, names []string, capability string) {
	sb.WriteString("#if ")
	for i, name := range names {
		if i > 0 {
			sb.WriteString(" || ")
		}
		fmt.Fprintf(sb, "defined(KIGUMI_OP_%s)", name)
	}
	sb.WriteString("\n")
	if len(names) == 1 {
		fmt.Fprintf(sb, "    case OP_%s: rt_panic(\"this build has no %s capability\"); return NULL;\n", names[0], capability)
	} else {
		for _, name := range names {
			fmt.Fprintf(sb, "    case OP_%s:\n", name)
		}
		fmt.Fprintf(sb, "        rt_panic(\"this build has no %s capability\");\n        return NULL;\n", capability)
	}
	sb.WriteString("#endif\n")
}

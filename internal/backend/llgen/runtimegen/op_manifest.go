package runtimegen

//go:generate env UPDATE=1 go test kigumi/internal/backend/vm -run TestRuntimeOpsImplemented

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// The manifest names where an op's C body lands; every body is emitted into
// this one generated translation unit under its capability guard.
const (
	stdDispatchFile       = "rt_std_dispatch.inc"
	stdDispatchBodiesFile = "std_dispatch_bodies.txt"
)

// VMKind says how the MIR VM runs one std op.
type VMKind string

const (
	// VMHost is a host accelerator registered under the op's keys.
	VMHost VMKind = "host"
	// VMStd is a primitive the VM's key dispatcher answers.
	VMStd VMKind = "std"
	// VMGap is an op only `kigumi build` runs; the reason says why.
	VMGap VMKind = "gap"
)

// VMImpl is one live op's VM implementation, as the VM's op table declares it.
type VMImpl struct {
	Kind   VMKind
	Symbol string // the VM function (or table) answering the op; empty for a gap
	Reason string // why a gap exists; empty otherwise
}

// OpBody locates the hand-written C body of one live op.
type OpBody struct {
	Name string
	// Section is the `@section` the body sits in, e.g. "prelude".
	Section string
	// GuardCap is the capability of the body's `@cap` group, "" when it has none.
	GuardCap string
	// GuardMacro is the `@cap` group's macro, "" when it has none.
	GuardMacro string
	// Macros are the KIGUMI_CAP_* macros the body itself mentions, which is how
	// an op outside a `@cap` group guards its own body.
	Macros []string
}

// evidence is every capability macro that gates the body.
func (b OpBody) evidence() []string {
	out := append([]string(nil), b.Macros...)
	if b.GuardMacro != "" {
		out = append(out, b.GuardMacro)
	}
	return out
}

var (
	capMacroRe = regexp.MustCompile(`\bKIGUMI_CAP_[A-Z0-9_]+\b`)
	// sectionNameRe keeps a prose `@section` from becoming a locator.
	sectionNameRe = regexp.MustCompile(`^[a-z0-9_.-]+( / [a-z0-9_.-]+)*$`)
)

// OpBodies indexes the hand-written C bodies of the embedded dispatch table by
// op name, after proving the table answers the registry and that each body is
// guarded by the capability the registry gives its op.
func OpBodies(ops []Op) (map[string]OpBody, error) { return opBodies(ops, stdDispatchBodies) }

func opBodies(ops []Op, text string) (map[string]OpBody, error) {
	blocks, err := parseStdDispatchBodies(text)
	if err != nil {
		return nil, err
	}
	if err := checkStdDispatchBlocks(ops, blocks); err != nil {
		return nil, err
	}
	index := map[string]OpBody{}
	section := ""
	record := func(name string, guard *stdDispatchBlock, body []string) {
		b := OpBody{Name: name, Section: section}
		if guard != nil {
			b.GuardCap, b.GuardMacro = guard.Text, guard.Macro
		}
		for _, line := range body {
			b.Macros = append(b.Macros, capMacroRe.FindAllString(line, -1)...)
		}
		index[name] = b
	}
	for i := range blocks {
		b := &blocks[i]
		switch b.Kind {
		case stdDispatchSection:
			if sectionNameRe.MatchString(b.Text) {
				section = b.Text
			}
		case stdDispatchOp:
			record(b.Names[0], nil, b.Body)
		case stdDispatchCap:
			for _, arm := range b.Arms {
				for _, name := range arm.Names {
					record(name, b, arm.Body)
				}
			}
		}
	}
	for _, op := range ops {
		if op.Retired {
			continue
		}
		body := index[op.Name]
		if op.Cap == "" {
			if ev := body.evidence(); len(ev) > 0 {
				return nil, fmt.Errorf("%s: op %s is guarded by %s but the registry gives it no capability", stdDispatchBodiesFile, op.Name, strings.Join(ev, ", "))
			}
			continue
		}
		if !guardsAny(body.evidence(), op.CapMacros) {
			return nil, fmt.Errorf("%s: op %s has capability %s but its body is guarded by none of %s", stdDispatchBodiesFile, op.Name, op.Cap, strings.Join(op.CapMacros, ", "))
		}
	}
	return index, nil
}

func guardsAny(have, want []string) bool {
	for _, h := range have {
		if slices.Contains(want, h) {
			return true
		}
	}
	return false
}

// ManifestEntry is one live op joined with its C body and VM implementation.
type ManifestEntry struct {
	Op   Op
	Body OpBody
	VM   VMImpl
}

// RuntimeManifest renders the runtime op manifest: one line per registry op
// naming its C body, capability, VM implementation and std keys. It fails when
// a live op has no C body, no std key or no VM implementation (a VM gap is an
// implementation only when it carries a reason), and when the VM table names an
// op the registry does not.
func RuntimeManifest(ops []Op, vm map[string]VMImpl) (string, error) {
	return runtimeManifest(ops, vm, stdDispatchBodies)
}

func runtimeManifest(ops []Op, vm map[string]VMImpl, bodies string) (string, error) {
	index, err := opBodies(ops, bodies)
	if err != nil {
		return "", err
	}
	live := map[string]bool{}
	for _, op := range ops {
		if !op.Retired {
			live[op.Name] = true
		}
	}
	var unknown []string
	for name := range vm {
		if !live[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return "", fmt.Errorf("VM op table names %s, which the registry has no live op for", strings.Join(unknown, ", "))
	}
	var sb strings.Builder
	sb.WriteString(manifestHeader)
	for _, op := range ops {
		if op.Retired {
			fmt.Fprintf(&sb, "op %d %s retired\n", op.ID, op.Name)
			continue
		}
		if len(op.Keys)+len(op.Suffixes)+len(op.Prefixes) == 0 {
			return "", fmt.Errorf("op %s has no key, suffix or prefix, so no std function resolves to it", op.Name)
		}
		impl, ok := vm[op.Name]
		if !ok {
			return "", fmt.Errorf("op %s has no VM implementation", op.Name)
		}
		if err := impl.validate(op.Name); err != nil {
			return "", err
		}
		sb.WriteString(manifestLine(op, index[op.Name], impl))
	}
	return sb.String(), nil
}

func (impl VMImpl) validate(name string) error {
	switch impl.Kind {
	case VMHost, VMStd:
		if impl.Symbol == "" {
			return fmt.Errorf("op %s: a %s implementation needs the VM function answering it", name, impl.Kind)
		}
		if impl.Reason != "" {
			return fmt.Errorf("op %s: only a gap carries a reason, not a %s implementation", name, impl.Kind)
		}
	case VMGap:
		if impl.Reason == "" {
			return fmt.Errorf("op %s: a VM gap needs the reason the VM cannot run it", name)
		}
		if impl.Symbol != "" {
			return fmt.Errorf("op %s: a VM gap has no implementation, so it names no function", name)
		}
	default:
		return fmt.Errorf("op %s: %q is not a VM implementation kind", name, impl.Kind)
	}
	return nil
}

func manifestLine(op Op, body OpBody, impl VMImpl) string {
	kind := "primitive"
	if op.Provider {
		kind = "provider"
	}
	cap := orDash(op.Cap)
	guard := orDash(guardMacro(op, body))
	var sb strings.Builder
	fmt.Fprintf(&sb, "op %d %s %s cap=%s c=%s:%s guard=%s vm=%s:%s",
		op.ID, op.Name, kind, cap, stdDispatchFile, orDash(sectionToken(body.Section)), guard, impl.Kind, orDash(impl.Symbol))
	if impl.Reason != "" {
		sb.WriteString(" reason=")
		sb.WriteString(strconv.Quote(impl.Reason))
	}
	for _, group := range []struct {
		name string
		keys []string
	}{{"keys", op.Keys}, {"suffixes", op.Suffixes}, {"prefixes", op.Prefixes}} {
		if len(group.keys) > 0 {
			fmt.Fprintf(&sb, " %s=%s", group.name, strings.Join(group.keys, ","))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

// guardMacro is the capability macro gating the body: the `@cap` group's, or
// the one an unguarded body mentions for itself.
func guardMacro(op Op, body OpBody) string {
	if body.GuardMacro != "" {
		return body.GuardMacro
	}
	for _, macro := range body.Macros {
		if slices.Contains(op.CapMacros, macro) {
			return macro
		}
	}
	return ""
}

// sectionToken renders a `@section` comment as one field: "os / fs" is "os/fs".
func sectionToken(section string) string {
	return strings.NewReplacer(" / ", "/", " ", "-").Replace(section)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

const manifestHeader = `# Runtime op manifest: every stdops.reg op with its C body, capability, VM
# implementation and std keys. Generated by TestRuntimeOpsImplemented; do not edit.
# Regenerate: UPDATE=1 go test ./internal/backend/vm -run TestRuntimeOpsImplemented
#
# op ID NAME KIND cap=<capability|-> c=<generated C file>:<section> guard=<macro|->
#   vm=<host|std|gap>:<VM function|-> [reason="..."] keys=... suffixes=... prefixes=...
# A C body is one case OP_<NAME> of rt_std_id in rt_std_dispatch.inc, written in the
# named section of std_dispatch_bodies.txt; guard is the capability macro compiling it.
# vm=host is a host accelerator registered under the op's keys, vm=std a primitive the
# VM's key dispatcher answers, and vm=gap an op only kigumi build runs.
`

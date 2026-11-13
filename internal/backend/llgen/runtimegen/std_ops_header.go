package runtimegen

import (
	"fmt"
	"strings"
)

// Op is one std op as the runtime artifacts see it: the registry facts that
// fix `enum std_op`, the KIGUMI_OP_ case guards, the key resolver, and the
// capability that guards the op's C body.
type Op struct {
	ID      int
	Name    string
	Retired bool
	// Provider marks an op needing a Target/Capability (the VM's host table).
	Provider bool
	// Cap is the registry capability, "" for an op every target provides.
	Cap string
	// CapMacros are Cap's macro and the macros of the capabilities Cap needs;
	// a capability-guarded C body mentions one of them.
	CapMacros                []string
	Keys, Suffixes, Prefixes []string
}

// StdOpHeader renders the registry as rt_std_ops.h: `enum std_op`, the retired case labels, the live ops' KIGUMI_OP_<NAME>
// case guards (all on without KIGUMI_OP_SELECT) and rt_std_op_of, the key resolver that mirrors ops.StdOpFor.
func StdOpHeader(ops []Op) string {
	var sb strings.Builder
	sb.WriteString("// Generated from internal/compiler/mir/ops/stdops.reg by TestStdOpHeaderFresh; do not edit.\n")
	sb.WriteString("#ifndef KIGUMI_RT_STD_OPS_H\n#define KIGUMI_RT_STD_OPS_H\n#ifndef KIGUMI_FREESTANDING\n#include <string.h>\n#endif\n")
	sb.WriteString("enum std_op {\n")
	for _, op := range ops {
		fmt.Fprintf(&sb, "    OP_%s = %d,\n", op.Name, op.ID)
	}
	sb.WriteString("};\n")
	writeRetiredCases(&sb, ops)
	sb.WriteString("#ifndef KIGUMI_OP_SELECT\n")
	for _, op := range ops {
		if !op.Retired {
			fmt.Fprintf(&sb, "#define %s\n", OpMacro(op.Name))
		}
	}
	sb.WriteString("#define KIGUMI_STD_DYNAMIC\n#endif\n#ifdef KIGUMI_STD_DYNAMIC\n")
	sb.WriteString("static int rt_std_op_of(const char* key) {\n    const char* k = key;\n    if (!strncmp(k, \"std/\", 4)) k += 4;\n")
	for _, op := range ops {
		writeMatch(&sb, "    ", "k", op.Keys, op.Name)
	}
	sb.WriteString("    const char* dot = strrchr(k, '.');\n    if (dot) {\n        const char* m = dot + 1;\n")
	for _, op := range ops {
		writeMatch(&sb, "        ", "m", op.Suffixes, op.Name)
	}
	sb.WriteString("    }\n")
	for _, op := range ops {
		for _, p := range op.Prefixes {
			fmt.Fprintf(&sb, "    if (!strncmp(k, %q, %d)) return OP_%s;\n", p, len(p), op.Name)
		}
	}
	sb.WriteString("    return -1;\n}\n#endif\n#endif\n")
	return sb.String()
}

// writeRetiredCases defines the case labels of retired ops so rt_std_id keeps -Wswitch complete.
func writeRetiredCases(sb *strings.Builder, ops []Op) {
	var labels []string
	for _, op := range ops {
		if op.Retired {
			labels = append(labels, "case OP_"+op.Name+":")
		}
	}
	if len(labels) == 0 {
		labels = []string{"default:"}
	}
	fmt.Fprintf(sb, "#define RT_STD_OPS_RETIRED_CASES %s\n", strings.Join(labels, " "))
}

func writeMatch(sb *strings.Builder, indent, subject string, values []string, op string) {
	if len(values) == 0 {
		return
	}
	conds := make([]string, len(values))
	for i, v := range values {
		conds[i] = fmt.Sprintf("!strcmp(%s, %q)", subject, v)
	}
	fmt.Fprintf(sb, "%sif (%s) return OP_%s;\n", indent, strings.Join(conds, " || "), op)
}

// OpMacro is the -D name that compiles the runtime case of the op named name.
func OpMacro(name string) string { return "KIGUMI_OP_" + name }

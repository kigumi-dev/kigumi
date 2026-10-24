package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

//go:generate env UPDATE=1 go test -run TestStdOpHeaderFresh .

// StdOpHeader renders the registry as rt_std_ops.h: the `enum std_op` the
// runtime dispatches on and rt_std_op_of, the string-key resolver for dynamic calls.
func StdOpHeader(reg *mir.StdOpRegistry) string {
	var sb strings.Builder
	sb.WriteString("// Generated from internal/mir/stdops.reg by TestStdOpHeaderFresh; do not edit.\n")
	sb.WriteString("#ifndef KIGUMI_RT_STD_OPS_H\n#define KIGUMI_RT_STD_OPS_H\n")
	sb.WriteString("enum std_op {\n")
	for _, op := range reg.Ops {
		fmt.Fprintf(&sb, "    OP_%s = %d,\n", op.Name, op.ID)
	}
	sb.WriteString("};\n")
	writeRetiredCases(&sb, reg)
	sb.WriteString("static int rt_std_op_of(const char* key) {\n    const char* k = key;\n    if (!strncmp(k, \"std/\", 4)) k += 4;\n")
	for _, op := range reg.Ops {
		writeMatch(&sb, "k", op.Keys, op.Name)
	}
	sb.WriteString("    const char* dot = strrchr(k, '.');\n    const char* m = dot ? dot + 1 : k;\n")
	for _, op := range reg.Ops {
		writeMatch(&sb, "m", op.Suffixes, op.Name)
	}
	for _, op := range reg.Ops {
		for _, p := range op.Prefixes {
			fmt.Fprintf(&sb, "    if (!strncmp(k, %q, %d)) return OP_%s;\n", p, len(p), op.Name)
		}
	}
	sb.WriteString("    return -1;\n}\n#endif\n")
	return sb.String()
}

// writeRetiredCases defines the case labels of retired ops so rt_std_id keeps -Wswitch complete.
func writeRetiredCases(sb *strings.Builder, reg *mir.StdOpRegistry) {
	var labels []string
	for _, op := range reg.Ops {
		if op.Retired {
			labels = append(labels, "case OP_"+op.Name+":")
		}
	}
	if len(labels) == 0 {
		labels = []string{"default:"}
	}
	fmt.Fprintf(sb, "#define RT_STD_OPS_RETIRED_CASES %s\n", strings.Join(labels, " "))
}

func writeMatch(sb *strings.Builder, subject string, values []string, op string) {
	if len(values) == 0 {
		return
	}
	conds := make([]string, len(values))
	for i, v := range values {
		conds[i] = fmt.Sprintf("!strcmp(%s, %q)", subject, v)
	}
	fmt.Fprintf(sb, "    if (%s) return OP_%s;\n", strings.Join(conds, " || "), op)
}

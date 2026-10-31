package llgen

import (
	"regexp"
	"slices"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

var builtinCompare = regexp.MustCompile(`!strcmp\(name, "([^"]+)"\)`)

func TestRuntimeBuiltinsMatchTheRegistry(t *testing.T) {
	src := RuntimeSource()
	start := regexp.MustCompile(`val rt_builtin\(`).FindStringIndex(src)
	if start == nil {
		t.Fatal("rt_builtin not found in the runtime")
	}
	body := src[start[0]:]
	if end := regexp.MustCompile(`\n}\n`).FindStringIndex(body); end != nil {
		body = body[:end[1]]
	}
	var got, want []string
	for _, m := range builtinCompare.FindAllStringSubmatch(body, -1) {
		got = append(got, m[1])
	}
	for _, name := range opdef.BuiltinNames() {
		if spec, _ := opdef.BuiltinFor(name); spec.Native == opdef.NativeRuntime {
			want = append(want, name)
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("rt_builtin handles %v, the registry says the runtime handles %v", got, want)
	}
}

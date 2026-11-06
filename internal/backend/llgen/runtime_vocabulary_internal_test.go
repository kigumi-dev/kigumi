package llgen

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

var capMacroRe = regexp.MustCompile(`KIGUMI_CAP_[A-Z0-9_]+`)

// runtimeSources is every runtime text, including the target-only translation units.
func runtimeSources() string {
	var sb strings.Builder
	seen := map[string]bool{}
	for _, t := range [][2]string{{"posix", "linux"}, {"none", "uefi"}} {
		for _, f := range RuntimeFiles(t[0], t[1]) {
			if !seen[f.Name] {
				seen[f.Name] = true
				sb.WriteString(f.Src)
			}
		}
	}
	return sb.String()
}

func TestRuntimeGuardsAreTheVocabulary(t *testing.T) {
	src := runtimeSources()
	want := map[string]bool{}
	for _, c := range opdef.Vocabulary() {
		want[c.Macro()] = true
	}
	seen := map[string]bool{}
	for _, m := range capMacroRe.FindAllString(src, -1) {
		seen[m] = true
		if !want[m] {
			t.Errorf("runtime guards on %s, which no vocabulary capability names", m)
		}
	}
	for m := range want {
		if !seen[m] {
			t.Errorf("vocabulary macro %s appears nowhere in the runtime", m)
		}
	}
}

func TestRuntimeFragmentsAreTheVocabulary(t *testing.T) {
	var got, want []string
	for _, f := range RuntimeFiles("posix", "linux") {
		if strings.HasPrefix(f.Name, "rt_cap_") {
			got = append(got, f.Name)
		}
	}
	for _, c := range opdef.Vocabulary() {
		if c.HasSource() {
			want = append(want, c.SourceFile())
		}
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("runtime fragments %v, vocabulary expects %v", got, want)
	}
}

// A type is looked up by short name or, where a user type could share it, by its std/pkg. qualified name.
var namedTypeRe = regexp.MustCompile(`(?:rt_make_record|rt_find_type)\("(?:std/[a-z]+\.)?([A-Za-z0-9_]+)"`)

// Tuple2 is built for a Map element, whose loop variable's own type keeps it.
func TestRuntimeNamedTypesAreReachabilityRoots(t *testing.T) {
	named := opdef.RuntimeNamedTypes()
	used := map[string]bool{}
	for _, m := range namedTypeRe.FindAllStringSubmatch(RuntimeSource(), -1) {
		used[m[1]] = true
		if m[1] != "Tuple2" && !named[m[1]] {
			t.Errorf("rt_core.c looks up type %s by name, but reachability does not keep it for any op", m[1])
		}
	}
	for name := range named {
		if !used[name] {
			t.Errorf("reachability keeps %s for the runtime, which never looks it up by name", name)
		}
	}
}

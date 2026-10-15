package llgen

import (
	"regexp"
	"slices"
	"testing"

	"kigumi/internal/mir"
)

var capMacroRe = regexp.MustCompile(`KIGUMI_CAP_[A-Z0-9_]+`)

func TestRuntimeGuardsAreTheVocabulary(t *testing.T) {
	src := RuntimeSource() + runtimeSignal + runtimeFreestandingHeader
	want := map[string]bool{}
	for _, c := range mir.Vocabulary() {
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
	for _, f := range capFiles {
		got = append(got, f.Name)
	}
	for _, c := range mir.Vocabulary() {
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
var namedTypeRe = regexp.MustCompile(`(?:make_record|find_type)\("(?:std/[a-z]+\.)?([A-Za-z0-9_]+)"`)

// Tuple2 is built for a Map element, whose loop variable's own type keeps it.
func TestRuntimeNamedTypesAreReachabilityRoots(t *testing.T) {
	named := mir.RuntimeNamedTypes()
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

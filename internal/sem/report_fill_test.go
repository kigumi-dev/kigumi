package sem

import "testing"

// mirrors internal/driver's E994 template (HeaderImportSkipped) rather than a hand-duplicated string.
func TestFillTemplate(t *testing.T) {
	got := FillTemplate("importing `{path}` skipped {n} declaration{s}: {list}", "h.h", 3, Plural(3), "a; b; c")
	want := "importing `h.h` skipped 3 declaration" + "s" + ": a; b; c"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if Plural(1) != "" {
		t.Errorf("Plural(1) = %q, want \"\"", Plural(1))
	}
	if Plural(2) != "s" {
		t.Errorf("Plural(2) = %q, want \"s\"", Plural(2))
	}
}

// E1430 and E1432's call site in derive_gen.go always passes (Name, iface, detail);
// each template's placeholders must appear in that same order.
func TestDeriveGeneratorCodeTemplateArgOrder(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"derive-generator-failed", "`MyType`'s `Ord` derive generator failed: detail"},
		{"derive-generator-unsupported", "no derive generator is registered for `MyType`'s `Ord`"},
	}
	for _, c := range cases {
		code, ok := CodeByName(c.name)
		if !ok {
			t.Fatalf("%s: no such code", c.name)
		}
		got := FillTemplate(code.Template, "MyType", "Ord", "detail")
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/token"
)

func isrModule(t *testing.T, manifest, src string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"isr\"\n    kigumi: \"0.1\"\n}\n"+manifest), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const isrWideSrc = `fn widen(a: i128) -> i128 {
    a.wrappingAdd(1)
}

export(interrupt) {
    pub fn tick() -> Unit {
        let a: i128 = 7
        _ = widen(a)
    }
}

fn main() -> Unit! {}
`

// The sem summary of tick is clean (wrappingAdd carries an inert contract), but the i128 runtime path boxes.
func TestInterruptLoweringFinalCheck(t *testing.T) {
	t.Parallel()
	only, err := driver.CheckOnly(isrModule(t, "", isrWideSrc))
	if err != nil || only.HasErrors() {
		t.Fatalf("the checker alone must accept tick: %v\n%s", err, only.Render())
	}
	res, err := driver.Check(isrModule(t, "", isrWideSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	for _, want := range []string{"E1090", "handler `tick` lowers to code that", "calls `widen`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("the lowering of tick must be rejected after MIR (missing %q):\n%s", want, out)
		}
	}
	ed, err := driver.CheckForEditor(isrModule(t, "", isrWideSrc))
	if err != nil || !strings.Contains(ed.Render(), "E1090") {
		t.Fatalf("the editor path must run the same final check: %v\n%s", err, ed.Render())
	}
}

const isrTickSrc = `export(interrupt) {
    pub fn tick() -> Unit {}
}

fn main() -> Unit! {}
`

func TestInterruptManifestConfig(t *testing.T) {
	t.Parallel()
	res, err := driver.Check(isrModule(t, "Interrupt { allow: \"mmio uart.ack\" }\nInterrupt { handler: \"isr.gone\" }\n", isrTickSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	for _, want := range []string{"E1091", "the root interrupt configuration names `mmio uart.ack`, which matches no reviewed contract", "E1092", "names handler `isr.gone`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("mod.kg's Interrupt records must reach the checker (missing %q):\n%s", want, out)
		}
	}
	if res, err := driver.Check(isrModule(t, "Interrupt { allow: \"\" }\n", isrTickSrc)); err != nil || res.HasErrors() {
		t.Fatalf("an empty root record permits ordinary computation: %v\n%s", err, res.Render())
	}
}

func TestInterruptManifestRecords(t *testing.T) {
	cases := []struct{ text, want string }{
		{"Interrupt { allow: \"sync q width=32\" }", "names its width= and impl="},
		{"Interrupt { allow: \"inert\" }", "ordinary computation needs no permission"},
		{"Interrupt { allow: \"\" }\nInterrupt { allow: \"\" }", "only one Interrupt record may omit"},
		{"Interrupt { handler: \"tick\" }", "must be written `pkg/path.name`"},
		{"Interrupt { handler: \"a.b, a.b\" }", "configured twice"},
	}
	for _, c := range cases {
		src := "Module {\n    name: \"isr\"\n    kigumi: \"0.1\"\n}\n" + c.text + "\n"
		_, ds := driver.ParseModFile(token.NewFile("mod.kg", []byte(src)))
		if len(ds) == 0 || !strings.Contains(ds[0].Msg, c.want) {
			t.Fatalf("%q: want a diagnostic mentioning %q, got %v", c.text, c.want, ds)
		}
	}
	mf, ds := driver.ParseModFile(token.NewFile("mod.kg", []byte("Module {\n    name: \"isr\"\n    kigumi: \"0.1\"\n}\nInterrupt { allow: \"sync  q.n impl=cas   width=32 ; mmio uart.ack\" }\nInterrupt { handler: \"isr.a, isr.b\", allow: \"mmio uart.ack\" }\n")))
	if len(ds) != 0 || strings.Join(mf.Interrupt.Root, "|") != "mmio uart.ack|sync q.n width=32 impl=cas" || len(mf.Interrupt.Handlers["isr.b"]) != 1 {
		t.Fatalf("records must canonicalize and share a handler record: %+v %v", mf.Interrupt, ds)
	}
}

func TestInterruptConfigWithoutHandlers(t *testing.T) {
	t.Parallel()
	res, err := driver.Check(isrModule(t, "Interrupt { handler: \"isr.gone, elsewhere/pkg.h\" }\n", "fn main() -> Unit! {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if !strings.Contains(out, "E1092") || !strings.Contains(out, "names handler `isr.gone`") || strings.Contains(out, "elsewhere/pkg.h") {
		t.Fatalf("a handler key into a loaded package must be checked even with no handler, and one into an unloaded package skipped:\n%s", out)
	}
}

package driver_test

import (
	"bytes"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/toolchain/driver"
)

// The default argument runs before the callee body and only borrows the
// resource arguments. When it fails, the caller still owns both resources and
// must release them in reverse creation order.
const argCleanupCommon = `
type Item = resource { label String }
fn Item.drop(move self) { print "drop ${self.label}" }
type Problem = { }
fn Problem.message(self) -> String { "failed" }
fn make(label: String) -> Item { print "make ${label}"; Item { label: label } }
fn tryMake(label: String) -> Item! {
    print "try ${label}"
    match label {
        "bad" -> { fail Problem {} }
        _ -> { Item { label: label } }
    }
}
fn stop() -> Int! { print "stop"; fail Problem {} }
fn ok() -> Int! { print "ok"; 7 }
fn collect(first: Item, third: Item, second: Int = stop()?) -> Void! { print "body" }
fn collectOk(first: Item, third: Item, second: Int = ok()?) -> Void! { print "body ${second}" }
`

func argCleanupProgram(scenario string) string {
	return argCleanupCommon + scenario + `
let result = run()
print "error=${result is Err(_)}"
`
}

func runArgCleanup(t *testing.T, src string) string {
	t.Helper()
	a, err := driver.Analyze(editorModule(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatal(a.Res.Render())
	}
	var out, errOut bytes.Buffer
	if code, err := vm.Run(a.Prog, vm.Options{}, &out, &errOut); err != nil || code != 0 {
		t.Fatalf("run: %v code=%d stderr=%s", err, code, errOut.String())
	}
	return out.String()
}

func TestArgumentFailureCleanupOrder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scenario string
		want     string
	}{
		{
			name: "default",
			scenario: `
fn run() -> Void! { collect(first: make("first"), third: make("third"))? }
`,
			want: "make first\nmake third\nstop\ndrop third\ndrop first\nerror=true\n",
		},
		{
			name: "named argument",
			scenario: `
fn run() -> Void! { collect(third: make("third"), first: make("first"))? }
`,
			want: "make third\nmake first\nstop\ndrop first\ndrop third\nerror=true\n",
		},
		{
			name: "explicit trailing argument",
			scenario: `
fn run() -> Void! { collect(first: make("first"), third: make("third"), second: stop()?) }
`,
			want: "make first\nmake third\nstop\ndrop third\ndrop first\nerror=true\n",
		},
		{
			name: "successful default",
			scenario: `
fn run() -> Void! { collectOk(first: make("first"), third: make("third"))? }
`,
			want: "make first\nmake third\nok\nbody 7\ndrop third\ndrop first\nerror=false\n",
		},
		{
			name: "assignment RHS failure keeps the old value",
			scenario: `
fn run() -> Void! {
    let mut held = make("held")
    held = tryMake("bad")?
}
`,
			want: "make held\ntry bad\ndrop held\nerror=true\n",
		},
		{
			name: "assignment RHS success replaces the old value",
			scenario: `
fn run() -> Void! {
    let mut held = make("held")
    held = tryMake("new")?
    print "after ${held.label}"
}
`,
			want: "make held\ntry new\ndrop held\nafter new\ndrop new\nerror=false\n",
		},
		{
			name: "joined arguments release newest first",
			scenario: `
fn pair(a: &Item, b: &Item) { print "${a.label}/${b.label}" }
fn run() -> Void! {
    pair(&(if true { make("a") } else { make("x") }), &(if true { make("b") } else { make("y") }))
}
`,
			want: "make a\nmake b\na/b\ndrop b\ndrop a\nerror=false\n",
		},
		{
			name: "join and loop drop each resource once",
			scenario: `
fn pair(a: &Item, b: &Item) { print "${a.label}/${b.label}" }
fn run() -> Void! {
    pair(&(if true { make("then") } else { make("else") }), &make("other"))
    let mut i = 0
    for i < 2 {
        pair(&make("a"), &make("b"))
        i = i + 1
    }
}
`,
			want: "make then\nmake other\nthen/other\ndrop then\ndrop other\nmake a\nmake b\na/b\ndrop b\ndrop a\nmake a\nmake b\na/b\ndrop b\ndrop a\nerror=false\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runArgCleanup(t, argCleanupProgram(tc.scenario))
			if got != tc.want {
				t.Errorf("output:\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

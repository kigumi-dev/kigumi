package mir

import "testing"

func TestKeptReferentDeclaredSignature(t *testing.T) {
	p, _ := srcProgram(t, `import cli from std/cli
fn flags(p: &mut cli.Parser, n: &mut Int) { p.flag("n", n) }
`)
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
}

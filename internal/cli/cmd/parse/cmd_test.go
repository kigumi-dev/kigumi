package parse_test

import (
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestHelp(t *testing.T) {
	code, out, errOut := clitest.Execute(t, "parse", "--help")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	clitest.Golden(t, "help.golden", out)
}

func TestDumpsTree(t *testing.T) {
	code, out, errOut := clitest.Execute(t, "parse", "../../../../examples/tour/main.kg")
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	clitest.Golden(t, "tour.golden", out)
}

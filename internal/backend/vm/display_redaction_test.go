package vm_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

// TestDebugRedactionHidesEveryPrivateField pins the rule both backends share: a private field never reaches the
// rendering. A record wider than the native descriptor's one-word mask hides every field, which is the only
// thing the runtime can do there and is what the VM does as well.
func TestDebugRedactionHidesEveryPrivateField(t *testing.T) {
	var fields, inits strings.Builder
	for i := range 65 {
		if i > 0 {
			inits.WriteString(", ")
		}
		if i == 64 {
			fields.WriteString("    hidden u32\n")
			inits.WriteString("hidden: 1234")
			continue
		}
		fmt.Fprintf(&fields, "    pub f%d u32\n", i)
		fmt.Fprintf(&inits, "f%d: %d", i, i)
	}
	src := fmt.Sprintf(`type Small = {
    hidden u32
    pub shown u32
}

type Wide = {
%s}

fn main() {
    print debug(&Small { hidden: 1, shown: 7 })
    print debug(&Wide { %s })
}
`, fields.String(), inits.String())
	prog := secretSanProgram(t, src)
	var out, errOut bytes.Buffer
	code, err := vm.Run(prog, vm.Options{}, &out, &errOut)
	if err != nil || code != 0 {
		t.Fatalf("vm: %v (exit %d)\n%s", err, code, errOut.String())
	}
	lines := strings.Split(out.String(), "\n")
	if want := "Small { hidden: <private>, shown: 7 }"; lines[0] != want {
		t.Errorf("small record = %q, want %q", lines[0], want)
	}
	wide := lines[1]
	if strings.Contains(wide, "1234") {
		t.Errorf("wide record leaked the private value: %q", wide)
	}
	if strings.Count(wide, "<private>") != 65 {
		t.Errorf("wide record rendered %d hidden fields, want all 65: %q", strings.Count(wide, "<private>"), wide)
	}
}

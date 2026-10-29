package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestByteBoxesAllocateNothingNatively(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, before string }{
		{"byte_box_noalloc", "            signed(&a)"},
		{"byte_box_callback_noalloc", "    _ = indirect(identity, -128)"},
		{"recoverable_noalloc_callback", "    invoke(noop)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fx := parseSemDriverFixture(t, "../../testdata/sem/"+c.name+".txtar")
			src := fx.files["main.kg"]
			if strings.Count(src, c.before) != 1 {
				t.Fatal("missing unique arming point")
			}
			src = strings.Replace(src, c.before, "    eprint \"armed\"\n"+c.before, 1)
			m := loadSweepModule(t, src, fx.opts["profile"])
			exe := buildFaultInjected(t, m)
			for n := 1; n <= 3; n++ {
				cmd := exec.Command(exe)
				cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_ARM=1", "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
				var out, errOut bytes.Buffer
				cmd.Stdout, cmd.Stderr = &out, &errOut
				if code := exitCodeOf(cmd.Run()); code != 0 || out.Len() != 0 || errOut.String() != "armed\n" {
					t.Fatalf("refusing allocation %d: exit %d, stdout=%q stderr=%q", n, code, out.String(), errOut.String())
				}
			}
		})
	}
}

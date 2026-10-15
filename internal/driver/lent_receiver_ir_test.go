package driver_test

import (
	"strings"
	"testing"
)

// A callee that never replaces its receiver gets a view over the place: no retain, no taking the content back, no store afterwards.
func TestLentReceiverBuildsNoWriteBack(t *testing.T) {
	ir := fixtureIR(t, "../../testdata/run/mut_self_lent_receiver.txtar")
	for _, name := range []string{"main.viaArgs", "main.bumpTwice"} {
		body := functionBody(t, ir, name)
		if !strings.Contains(body, "@rt_cell_init") {
			t.Errorf("%s hands its callee no cell:\n%s", name, body)
		}
		for _, banned := range []string{"@rt_cell_take", "@rt_setfield_swap", "@rt_setindex_swap"} {
			if strings.Contains(body, banned) {
				t.Errorf("%s still calls %s around a call that cannot replace its receiver", name, banned)
			}
		}
	}
	reset := functionBody(t, ir, "main.main")
	if !strings.Contains(reset, "@rt_cell_take") {
		t.Error("a call that replaces its receiver no longer takes the content back")
	}
}

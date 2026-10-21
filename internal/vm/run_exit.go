package vm

import (
	"fmt"
	"io"

	"kigumi/internal/diag"
)

// settleRun maps how the entry ended to the run's exit code: a panic prints and gives 2, host.exit gives its own code.
func settleRun(code int, err error, errOut io.Writer) (int, error) {
	if p, ok := err.(*Panic); ok {
		fmt.Fprintf(errOut, "%s: %s\n", diag.Prefix("panic"), p.Msg)
		return 2, nil
	}
	if x, ok := err.(*exitSignal); ok {
		return x.code, nil
	}
	return code, err
}

func (m *Machine) exitCode(res *obj) int {
	res = deref(res)
	if res != nil && res.k == kVariant && res.ent == m.variantOf(m.r.Types.ResultEnt(), "Err") {
		if code, ok := m.exitStatus(res.fields[0]); ok {
			return code
		}
		m.writeErr([]byte(fmt.Sprintf("%s: %s\n", diag.Prefix("error"), m.display(m.errorMessage(res.fields[0])))))
		return 1
	}
	return 0
}

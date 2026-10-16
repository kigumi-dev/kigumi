package explain

import (
	"fmt"
	"io"
	"strings"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
	"kigumi/internal/errors"
	"kigumi/internal/sem"

	"github.com/spf13/cobra"
)

func explainOp(cmd *cobra.Command, name, target string) error {
	m, err := shared.Load(cmd, target, false, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	m.EveryBody()
	res, err := driver.Check(m)
	if err != nil {
		return errors.WrapErr(err, "check module")
	}
	id, ok := findFn(res, name)
	if !ok {
		return &cliutil.UsageError{Err: errors.NewErrf("no function %q (write it as pkg/path.Name)", name)}
	}
	printOpSummary(cmd.OutOrStdout(), res, id, name)
	return nil
}

func findFn(r *sem.Result, name string) (sem.EntityID, bool) {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return 0, false
	}
	pkgPath, fnName := name[:i], name[i+1:]
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == sem.EntFn && e.Name == fnName && r.Packages[e.Pkg].Path == pkgPath {
			return sem.EntityID(id), true
		}
	}
	return 0, false
}

var opFactNames = []sem.OpFact{
	sem.FactAlloc, sem.FactRelease, sem.FactBlock, sem.FactSuspend,
	sem.FactPanic, sem.FactIO, sem.FactSync, sem.FactSpawn,
}

func printOpSummary(out io.Writer, r *sem.Result, id sem.EntityID, name string) {
	fmt.Fprintf(out, "%s\n", name)
	any := false
	for _, f := range opFactNames {
		ex := r.ExplainOpFact(id, f)
		if !ex.Maybe {
			continue
		}
		any = true
		fmt.Fprintf(out, "  may %s\n", f)
		for _, hop := range ex.Chain {
			fmt.Fprintf(out, "    %s\n", hop.Text)
		}
	}
	if !any {
		fmt.Fprint(out, "  no operation in this closed vocabulary\n")
	}
	if perms, ok := r.InterruptPermissionsOf(id); ok {
		printInterruptPermissions(out, perms)
	}
}

func printInterruptPermissions(out io.Writer, p sem.InterruptPermissions) {
	origin := "root common configuration"
	if p.Own {
		origin = "handler's own configuration"
	}
	fmt.Fprintf(out, "  interrupt handler, checked under the %s\n", origin)
	fmt.Fprintf(out, "    permits: %s\n", permList(p.Permits))
	fmt.Fprintf(out, "    root limit: %s\n", permList(p.RootLimit))
	fmt.Fprintf(out, "    reaches: %s\n", permList(p.Reached))
	if p.Unverified {
		fmt.Fprint(out, "    rests on code with no reviewed interrupt contract\n")
	}
}

func permList(ids []string) string {
	if len(ids) == 0 {
		return "no MMIO or synchronization operation"
	}
	return strings.Join(ids, "; ")
}

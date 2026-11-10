package explain

import (
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/driver"
)

func explainOp(io session.IO, s session.Session, name, target string) error {
	m, err := s.Load(target, false, io.Err)
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
		return &exit.ValueError{Err: errors.NewErrf("no function %q (write it as pkg/path.Name)", name)}
	}
	printOpSummary(io.Out, res, id, name)
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

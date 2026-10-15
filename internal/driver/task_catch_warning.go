package driver

import (
	"fmt"

	"kigumi/internal/mir"
)

// The runtime keeps a task panic fatal there; this only says so.
func (a *Analysis) warnTaskPanicFatal(t Target, reach *mir.Reachability) {
	if t.OS == "" {
		t = HostTarget()
	}
	if wantsTaskCatch(t) {
		return
	}
	spawn, ok := mir.StdOpIdOf("TASK_SPAWN")
	if !ok {
		return
	}
	site, ok := reach.OpSites[spawn]
	if !ok {
		return
	}
	caller := a.Prog.ByEnt[site.Caller]
	if caller == nil {
		return
	}
	a.Res.WarnTaskPanicFatal(targetLabel(t), caller.File, site.Node)
}

func targetLabel(t Target) string {
	if t.Triple != "" {
		return t.Triple
	}
	return fmt.Sprintf("%s-%s", t.Arch, t.OS)
}

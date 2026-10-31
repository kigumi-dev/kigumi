package driver

import (
	"fmt"

	"kigumi/internal/compiler/mir"
)

// The runtime keeps a task panic fatal there; this only says so.
func (a *Analysis) warnTaskPanicFatal(t Target, reach *mir.Reachability) {
	if wantsTaskCatch(t) {
		return
	}
	site, ok := firstTaskSite(reach)
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

func firstTaskSite(reach *mir.Reachability) (mir.OpSite, bool) {
	for _, name := range []string{"TASK_SPAWN", "TASK_HANDOFF"} {
		if op, ok := mir.StdOpIdOf(name); ok {
			if site, ok := reach.OpSites[op]; ok {
				return site, true
			}
		}
	}
	return mir.OpSite{}, false
}

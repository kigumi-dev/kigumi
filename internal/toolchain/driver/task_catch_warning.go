package driver

import (
	"fmt"

	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	buildconfig "kigumi/internal/toolchain/config"
)

// The runtime keeps a task panic fatal there; this only says so.
func (a *Analysis) warnTaskPanicFatal(t buildconfig.Target, reach *mirreach.Reachability) {
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

func targetLabel(t buildconfig.Target) string {
	if t.Triple != "" {
		return t.Triple
	}
	return fmt.Sprintf("%s-%s", t.Arch, t.OS)
}

func firstTaskSite(reach *mirreach.Reachability) (mirreach.OpSite, bool) {
	for _, name := range []string{"TASK_SPAWN", "TASK_HANDOFF"} {
		if op, ok := opdef.StdOpIdOf(name); ok {
			if site, ok := reach.OpSites[op]; ok {
				return site, true
			}
		}
	}
	return mirreach.OpSite{}, false
}

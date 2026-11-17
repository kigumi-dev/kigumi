package driver

import (
	"kigumi/internal/compiler/mir"
	mirreach "kigumi/internal/compiler/mir/reach"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
)

// reportSimdShapes reports whether reach keeps a body whose SIMD operation only an instance can lower, adding E1713
// for each unless an earlier error, such as E976 refusing the instance, already explains it.
func reportSimdShapes(res *sem.Result, prog *mir.Program, reach *mirreach.Reachability) bool {
	claims := mirverify.CheckSimdShapes(prog, reach)
	if !res.HasErrors() {
		for _, c := range claims {
			res.AppendDiag(c.File, c.Diag)
		}
	}
	return len(claims) > 0
}

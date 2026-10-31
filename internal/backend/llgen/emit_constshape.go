package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// Records the shape without reserving storage: an inline ADT needs none unless it crosses.
func (e *emitter) variantShape(in midir.Inst, keys map[midir.LocalID]string) (string, bool) {
	d := e.vdesc(in.Ent)
	if len(in.Args) == 0 {
		e.constShape[d] = constVal{vdesc: d}
		return d, true
	}
	args := make([]string, 0, len(in.Args))
	for _, a := range in.Args {
		k, ok := keys[a]
		if !ok {
			return "", false
		}
		args = append(args, k)
	}
	key := d + "(" + strings.Join(args, ",") + ")"
	e.constShape[key] = constVal{vdesc: d, args: args}
	return key, true
}

func (e *emitter) commitConst(key string) {
	cv, ok := e.constShape[key]
	if !ok {
		return
	}
	for _, a := range cv.args {
		e.commitConst(a)
	}
	e.registerConst(key, cv)
}

func fixedShape(shape map[midir.LocalID]string, l midir.LocalID) string {
	k, ok := shape[l]
	if !ok {
		panic("llgen: mir marks a crossing fixed whose value has no immortal shape")
	}
	return k
}

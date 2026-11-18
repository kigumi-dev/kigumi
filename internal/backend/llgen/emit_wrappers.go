package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

type wrapperKind uint8

const (
	wrapAdapter wrapperKind = iota
	wrapBind
	wrapMethod
	wrapEntry
	wrapKinds
)

type wrapperKey struct {
	f    *midir.Func
	kind wrapperKind
	open int
}

func (e *emitter) useAdapter(f *midir.Func) string {
	e.wrapNeed[wrapperKey{f, wrapAdapter, 0}] = true
	return e.adapterSym(f)
}

func (e *emitter) useBind(f *midir.Func, open int) string {
	e.wrapNeed[wrapperKey{f, wrapBind, open}] = true
	return e.bindSym(f, open)
}

func (e *emitter) useEntry(f *midir.Func) string {
	e.wrapNeed[wrapperKey{f, wrapEntry, 0}] = true
	return e.sym(f.Entry)
}

func (e *emitter) useMethodAdapter(f *midir.Func) string {
	e.wrapNeed[wrapperKey{f, wrapMethod, 0}] = true
	return e.methodAdapterSym(f)
}

func (e *emitter) wrappable(f *midir.Func, k wrapperKind) bool {
	isFn := e.r.Entity(f.Ent).Kind == sem.EntFn
	if f.Async || isFn && e.r.Fn(f.Ent).Naked {
		return false
	}
	if e.reachable != nil && !e.reachable.Keep(f.Ent) {
		return false
	}
	switch k {
	case wrapBind:
		return isFn && e.r.Fn(f.Ent).Abi == ""
	case wrapMethod:
		return isFn && e.r.Fn(f.Ent).Recv != sem.RecvNone
	}
	return true
}

// Runs to a fixed point because a wrapper body can reference more wrappers.
func (e *emitter) emitWrappers() string {
	var sb strings.Builder
	for {
		e.markTypeDrops()
		progress := false
		for _, f := range e.p.Funcs {
			for k := wrapAdapter; k < wrapKinds; k++ {
				limit := 0
				if k == wrapBind && e.r.Entity(f.Ent).Kind == sem.EntFn {
					limit = len(e.r.Fn(f.Ent).ConstParams)
				}
				for open := 0; open <= limit; open++ {
					key := wrapperKey{f, k, open}
					if !e.wrapNeed[key] || e.wrapDone[key] || !e.wrappable(f, k) {
						continue
					}
					e.wrapDone[key] = true
					progress = true
					e.tmp = 0
					switch k {
					case wrapAdapter:
						sb.WriteString(e.adapter(f))
					case wrapBind:
						sb.WriteString(e.bindAdapter(f, open))
					case wrapMethod:
						sb.WriteString(e.methodAdapter(f))
					case wrapEntry:
						sb.WriteString(e.function(f.Entry))
					}
				}
			}
		}
		if e.emitDescAdapters(&sb) {
			progress = true
		}
		if !progress {
			return sb.String()
		}
	}
}

func (e *emitter) dataTypes() []sem.EntityID {
	var ents []sem.EntityID
	for id := 1; id < len(e.r.Entities); id++ {
		ent := sem.EntityID(id)
		if e.r.Entity(ent).Kind != sem.EntType {
			continue
		}
		if e.reachable != nil && !e.reachable.KeepType(ent) {
			continue
		}
		ents = append(ents, ent)
	}
	return ents
}

func (e *emitter) typeDrop(info *sem.TypeDeclInfo) string {
	if info.Drop == 0 {
		return "null"
	}
	if e.p.ByEnt[info.Drop] != nil {
		return e.useDescAdapter(info.Drop, 0)
	}
	if ownquery.IsStdPrimitive(e.p.R, info.Drop) {
		e.stdDrops[info.Drop] = true
		return e.stdAdapterSym(info.Drop)
	}
	return "null"
}

func (e *emitter) markTypeDrops() {
	for _, ent := range e.dataTypes() {
		e.typeDrop(e.r.TypeDecl(ent))
	}
}

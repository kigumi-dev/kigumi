package sem

import (
	"maps"
	"slices"
	"strings"

	"kigumi/internal/syntax"
)

// ownsHandlerKey reports whether module owns the package a "pkg/path.name" key points into.
func ownsHandlerKey(module, key string) bool {
	i := strings.LastIndexByte(key, '.')
	return i > 0 && (key[:i] == module || strings.HasPrefix(key, module+"/"))
}

// depRecords are the permission sets that dependency records declare for the handler key.
func (r *Result) depRecords(key string) []map[string]bool {
	if r.mod == nil {
		return nil
	}
	var out []map[string]bool
	for _, d := range r.mod.DepInterrupt {
		if ids, ok := d.Config.Handlers[key]; ok && ownsHandlerKey(d.Module, key) {
			out = append(out, idSet(ids))
		}
	}
	return out
}

// checkInterruptConfig reports what the interrupt records get wrong, whether or not a handler they name exists in this build.
func (r *Result) checkInterruptConfig(handlers []EntityID) {
	cfg := r.interruptConfig()
	root := idSet(cfg.Root)
	sf, sn, siteOK := r.interruptConfigSite(handlers)
	at := func(key string) (FileID, syntax.NodeID, bool) {
		for _, h := range handlers {
			if r.handlerKey(h) == key {
				return r.Entities[h].File, r.Entities[h].Node, true
			}
		}
		return sf, sn, siteOK
	}
	if siteOK {
		r.checkIsrIds(sf, sn, "the root `Interrupt` record", cfg.Root, nil)
	}
	keys := slices.Sorted(maps.Keys(cfg.Handlers))
	for _, k := range keys {
		if f, n, ok := at(k); ok {
			r.checkIsrIds(f, n, "the `Interrupt` record for handler `"+k+"`", cfg.Handlers[k], root)
		}
	}
	if r.mod != nil {
		for _, d := range r.mod.DepInterrupt {
			keys = append(keys, r.checkDepInterrupt(d, root, at)...)
		}
	}
	r.checkInterruptConfigKeys(handlers, keys, sf, sn, siteOK)
}

// checkDepInterrupt validates one dependency's handler records (its common record is its own root limit and is ignored) and returns the handler keys it legitimately names.
func (r *Result) checkDepInterrupt(d DepInterrupt, root map[string]bool, at func(string) (FileID, syntax.NodeID, bool)) []string {
	var keys []string
	for _, k := range slices.Sorted(maps.Keys(d.Config.Handlers)) {
		f, n, ok := at(k)
		if !ok {
			continue
		}
		if !ownsHandlerKey(d.Module, k) {
			r.errAt(f, n, cInterruptConfigDep, d.Module, "names handler `"+k+"`, which belongs to another module")
			continue
		}
		r.checkIsrIds(f, n, "the `Interrupt` record of dependency `"+d.Module+"` for handler `"+k+"`", d.Config.Handlers[k], root)
		keys = append(keys, k)
	}
	return keys
}

// checkIsrIds rejects ids that match no reviewed contract and, when limit is set, ids outside it.
func (r *Result) checkIsrIds(f FileID, n syntax.NodeID, subject string, ids []string, limit map[string]bool) {
	for _, id := range ids {
		switch {
		case !r.isrKnown(id):
			r.errAt(f, n, cInterruptConfig, subject, id, "matches no reviewed contract")
		case limit != nil && !limit[id]:
			r.errAt(f, n, cInterruptConfig, subject, id, "is outside the root limit")
		}
	}
}

// checkInterruptConfigKeys rejects a handler key naming no export(interrupt) function of a package this build loaded.
func (r *Result) checkInterruptConfigKeys(handlers []EntityID, keys []string, f FileID, n syntax.NodeID, ok bool) {
	if !ok {
		return
	}
	known := map[string]bool{}
	for _, h := range handlers {
		known[r.handlerKey(h)] = true
	}
	loaded := map[string]bool{}
	for _, p := range r.Packages {
		loaded[p.Path] = true
	}
	slices.Sort(keys)
	for _, k := range slices.Compact(keys) {
		if i := strings.LastIndexByte(k, '.'); !known[k] && i > 0 && loaded[k[:i]] {
			r.errAt(f, n, cInterruptConfigHandler, k)
		}
	}
}

// interruptConfigSite is where a configuration finding is reported: the first handler, else the main module's first file.
func (r *Result) interruptConfigSite(handlers []EntityID) (FileID, syntax.NodeID, bool) {
	if len(handlers) > 0 {
		e := &r.Entities[handlers[0]]
		return e.File, e.Node, true
	}
	for _, p := range r.Packages {
		if p.Module == "" && !p.Std && len(p.Files) > 0 {
			f := p.Files[0]
			if p.HasScript {
				f = p.Entry
			}
			return f, r.tree(f).Root, true
		}
	}
	return 0, 0, false
}

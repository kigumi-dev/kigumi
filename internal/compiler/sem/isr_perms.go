package sem

import (
	"slices"
	"strings"
)

// isrScope is one handler's resolved permission set and every node's verdict under it.
type isrScope struct {
	handler EntityID
	// own is true when the handler has its own configuration record rather than the root common one.
	own     bool
	permits map[string]bool
	list    []string
	limit   []string
	safe    map[EntityID]bool
}

// InterruptPermissions is a handler's resolved configuration, for explain --fn.
type InterruptPermissions struct {
	Own       bool
	Permits   []string
	RootLimit []string
	// Reached are the sync/MMIO contract identities the handler's summary reaches, permitted or not.
	Reached []string
	// Unverified marks a summary that rests on code with no reviewed contract.
	Unverified bool
}

func (r *Result) handlerKey(id EntityID) string {
	return r.Packages[r.Entities[id].Pkg].Path + "." + r.Entities[id].Name
}

func (r *Result) interruptConfig() InterruptConfig {
	if r.mod == nil {
		return InterruptConfig{}
	}
	return r.mod.Interrupt
}

func (r *Result) isrKnown(id string) bool {
	c := r.isrContracts()
	return c != nil && c.IsrKnown(id)
}

// resolveIsrPerms narrows the root limit by the handler's own record and by the records of the module that owns it.
func (r *Result) resolveIsrPerms(h EntityID) (bool, []string, []string) {
	cfg := r.interruptConfig()
	root := idSet(cfg.Root)
	key := r.handlerKey(h)
	names, own := cfg.Handlers[key]
	if !own {
		names = cfg.Root
	}
	deps := r.depRecords(key)
	var out []string
	for _, id := range names {
		if r.isrKnown(id) && root[id] && inEvery(deps, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	out = slices.Compact(out)
	limit := slices.Clone(cfg.Root)
	slices.Sort(limit)
	return own || len(deps) > 0, out, limit
}

func idSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func inEvery(sets []map[string]bool, id string) bool {
	for _, s := range sets {
		if !s[id] {
			return false
		}
	}
	return true
}

func (r *Result) InterruptPermissionsOf(id EntityID) (InterruptPermissions, bool) {
	e := &r.Entities[id]
	if e.Kind != EntFn || r.Fn(id).Export != "interrupt" {
		return InterruptPermissions{}, false
	}
	own, list, limit := r.resolveIsrPerms(id)
	s := r.opSummary(id)
	return InterruptPermissions{Own: own, Permits: list, RootLimit: limit, Reached: splitContracts(s.Contracts), Unverified: s.Untrusted}, true
}

// isrScopes resolves every handler's permissions, sharing one verdict map per distinct set.
func (r *Result) isrScopes(handlers, nodes []EntityID) []*isrScope {
	bySet := map[string]map[EntityID]bool{}
	var out []*isrScope
	for _, h := range handlers {
		own, list, limit := r.resolveIsrPerms(h)
		sc := &isrScope{handler: h, own: own, list: list, limit: limit, permits: map[string]bool{}}
		for _, id := range list {
			sc.permits[id] = true
		}
		key := strings.Join(list, ";")
		if bySet[key] == nil {
			bySet[key] = r.inferIsrSafe(nodes, sc)
		}
		sc.safe = bySet[key]
		out = append(out, sc)
	}
	return out
}

// configHelp names the configuration origin and the root limit for a handler diagnostic.
func (sc *isrScope) configHelp(name string) string {
	if !sc.own {
		return "`" + name + "` uses the root common interrupt configuration, which permits " + isrPermText(sc.list)
	}
	return "`" + name + "` uses its own interrupt configuration, which permits " + isrPermText(sc.list) + " within the root limit of " + isrPermText(sc.limit)
}

func isrPermText(ids []string) string {
	if len(ids) == 0 {
		return "no MMIO or synchronization operation"
	}
	return "`" + strings.Join(ids, "`, `") + "`"
}

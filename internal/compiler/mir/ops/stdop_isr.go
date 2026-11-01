package ops

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/isr"
)

var IsrContracts sem.IsrContracts = registryIsr{Registry}

type registryIsr struct{ reg *StdOpRegistry }

// IsrContract resolves key the way StdOpFor does, then through the bodiless groups; an op without an `isr` line has none.
func (r registryIsr) IsrContract(key string) (isr.Contract, bool) {
	key = strings.TrimPrefix(key, "std/")
	if id, ok := StdOpFor(key); ok && int(id) < len(r.reg.Ops) {
		c := r.reg.Ops[id].Isr
		return c, c.Kind != isr.None
	}
	for _, g := range r.reg.Bodiless {
		if g.Isr.Kind == isr.None {
			continue
		}
		for _, k := range g.Keys {
			if k == key {
				return g.Isr, true
			}
		}
		for _, p := range g.Prefixes {
			if strings.HasPrefix(key, p) {
				return g.Isr, true
			}
		}
	}
	return isr.Contract{}, false
}

func (r registryIsr) IsrKnown(id string) bool {
	for _, op := range r.reg.Ops {
		if op.Isr.ID == id && id != "" {
			return true
		}
	}
	for _, g := range r.reg.Bodiless {
		if g.Isr.ID == id && id != "" {
			return true
		}
	}
	return false
}

// IsrVersion hashes every contract, so a reviewed change re-keys the artifacts that relied on it.
func (r registryIsr) IsrVersion() string {
	h := sha256.New()
	for _, op := range r.reg.Ops {
		fmt.Fprintf(h, "%s=%d:%s\n", op.Name, op.Isr.Kind, op.Isr.ID)
	}
	for _, g := range r.reg.Bodiless {
		fmt.Fprintf(h, "%s=%d:%s:%v:%v\n", g.Name, g.Isr.Kind, g.Isr.ID, g.Keys, g.Prefixes)
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

package sem

import (
	"fmt"
	"strings"

	"kigumi/internal/lang/isr"
)

// IsrContracts answers the reviewed contracts of bodiless std functions, keyed like stdKey.
type IsrContracts interface {
	IsrContract(key string) (isr.Contract, bool)
	IsrKnown(id string) bool
	IsrVersion() string
}

// DepInterrupt is one dependency module's Interrupt records.
type DepInterrupt struct {
	Module string
	Config isr.Config
}

// InterruptKey is the cache identity of the root configuration together with the dependencies' records.
func InterruptKey(root isr.Config, deps []DepInterrupt) string {
	var sb strings.Builder
	sb.WriteString(root.Canonical())
	for _, d := range deps {
		fmt.Fprintf(&sb, " dep %s {%s}", d.Module, d.Config.Canonical())
	}
	return sb.String()
}

func isrOpName(id string) string {
	words := strings.Fields(id)
	if len(words) < 2 {
		return id
	}
	return words[0] + " " + words[1]
}

func (r *Result) isrContracts() IsrContracts {
	if r.mod == nil {
		return nil
	}
	return r.mod.IsrContracts
}

// isrContractOf is a bodiless std function's reviewed contract; extern(C) and non-std code never have one.
func (r *Result) isrContractOf(id EntityID) (isr.Contract, bool) {
	c := r.isrContracts()
	e := &r.Entities[id]
	if c == nil || e.Kind != EntFn || r.Fn(id).Abi != "" || r.Fn(id).Body != 0 || !r.Packages[e.Pkg].Std {
		return isr.Contract{}, false
	}
	return c.IsrContract(r.stdKey(id))
}

package sem

import (
	"fmt"
	"slices"
	"strings"
)

// IsrContractKind is what a reviewed contract says a bodiless primitive does inside an interrupt handler.
type IsrContractKind uint8

const (
	IsrNone IsrContractKind = iota
	IsrInert
	IsrSync
	IsrMMIO
)

// IsrContract is one reviewed contract; ID is the canonical permission text for IsrSync and IsrMMIO, "" for IsrInert.
type IsrContract struct {
	Kind IsrContractKind
	ID   string
}

// IsrContracts answers the reviewed contracts of bodiless std functions, keyed like stdKey.
type IsrContracts interface {
	IsrContract(key string) (IsrContract, bool)
	IsrKnown(id string) bool
	IsrVersion() string
}

// InterruptConfigVersion moves when the meaning of InterruptConfig or of a contract identity changes.
const InterruptConfigVersion = 2

// InterruptConfig is the root module's resolved interrupt configuration: Root is both the root
// limit and the common configuration; Handlers narrows it for the handler keyed "pkg/path.name".
type InterruptConfig struct {
	Root     []string
	Handlers map[string][]string
}

func (c InterruptConfig) Canonical() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "v%d root=%s", InterruptConfigVersion, strings.Join(c.Root, ";"))
	keys := make([]string, 0, len(c.Handlers))
	for k := range c.Handlers {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(&sb, " %s=%s", k, strings.Join(c.Handlers[k], ";"))
	}
	return sb.String()
}

// DepInterrupt is one dependency module's Interrupt records.
type DepInterrupt struct {
	Module string
	Config InterruptConfig
}

// InterruptKey is the cache identity of the root configuration together with the dependencies' records.
func InterruptKey(root InterruptConfig, deps []DepInterrupt) string {
	var sb strings.Builder
	sb.WriteString(root.Canonical())
	for _, d := range deps {
		fmt.Fprintf(&sb, " dep %s {%s}", d.Module, d.Config.Canonical())
	}
	return sb.String()
}

var isrSyncFields = map[string]bool{"width": true, "impl": true, "nest": true}

// ParseIsrPermission canonicalizes one contract or permission text:
// `inert`, `mmio DEVICE.OP`, or `sync OP width=W impl=I [nest=C]`.
func ParseIsrPermission(text string) (IsrContract, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return IsrContract{}, fmt.Errorf("empty interrupt operation")
	}
	for _, w := range words {
		if strings.ContainsAny(w, ";,") {
			return IsrContract{}, fmt.Errorf("%q: an operation name cannot hold `;` or `,`", text)
		}
	}
	switch words[0] {
	case "inert":
		if len(words) != 1 {
			return IsrContract{}, fmt.Errorf("%q: `inert` takes nothing after it", text)
		}
		return IsrContract{Kind: IsrInert}, nil
	case "mmio":
		if len(words) != 2 || !strings.Contains(words[1], ".") {
			return IsrContract{}, fmt.Errorf("%q: want `mmio DEVICE.OPERATION`", text)
		}
		return IsrContract{Kind: IsrMMIO, ID: "mmio " + words[1]}, nil
	case "sync":
		return parseIsrSync(text, words)
	}
	return IsrContract{}, fmt.Errorf("%q: want `mmio ...` or `sync ...`", text)
}

func parseIsrSync(text string, words []string) (IsrContract, error) {
	if len(words) < 2 || strings.Contains(words[1], "=") {
		return IsrContract{}, fmt.Errorf("%q: want `sync OPERATION width=W impl=I [nest=C]`", text)
	}
	fields := map[string]string{}
	for _, w := range words[2:] {
		k, v, ok := strings.Cut(w, "=")
		if !ok || !isrSyncFields[k] || v == "" || fields[k] != "" {
			return IsrContract{}, fmt.Errorf("%q: %q is not one of width=, impl=, nest=, each given once", text, w)
		}
		fields[k] = v
	}
	if fields["width"] == "" || fields["impl"] == "" {
		return IsrContract{}, fmt.Errorf("%q: a synchronization operation names its width= and impl=", text)
	}
	id := "sync " + words[1] + " width=" + fields["width"] + " impl=" + fields["impl"]
	if n := fields["nest"]; n != "" {
		id += " nest=" + n
	}
	return IsrContract{Kind: IsrSync, ID: id}, nil
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
func (r *Result) isrContractOf(id EntityID) (IsrContract, bool) {
	c := r.isrContracts()
	e := &r.Entities[id]
	if c == nil || e.Kind != EntFn || r.Fn(id).Abi != "" || r.Fn(id).Body != 0 || !r.Packages[e.Pkg].Std {
		return IsrContract{}, false
	}
	return c.IsrContract(r.stdKey(id))
}

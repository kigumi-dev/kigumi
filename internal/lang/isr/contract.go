// Package isr is the interrupt-handler permission vocabulary that manifests, the checker and the op registry share.
package isr

import (
	"fmt"
	"strings"
)

// Kind is what a reviewed contract says a bodiless primitive does inside an interrupt handler.
type Kind uint8

const (
	None Kind = iota
	Inert
	Sync
	MMIO
)

// Contract is one reviewed contract; ID is the canonical permission text for Sync and MMIO, "" for Inert.
type Contract struct {
	Kind Kind
	ID   string
}

var syncFields = map[string]bool{"width": true, "impl": true, "nest": true}

// ParsePermission canonicalizes one contract or permission text:
// `inert`, `mmio DEVICE.OP`, or `sync OP width=W impl=I [nest=C]`.
func ParsePermission(text string) (Contract, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return Contract{}, fmt.Errorf("empty interrupt operation")
	}
	for _, w := range words {
		if strings.ContainsAny(w, ";,") {
			return Contract{}, fmt.Errorf("%q: an operation name cannot hold `;` or `,`", text)
		}
	}
	switch words[0] {
	case "inert":
		if len(words) != 1 {
			return Contract{}, fmt.Errorf("%q: `inert` takes nothing after it", text)
		}
		return Contract{Kind: Inert}, nil
	case "mmio":
		if len(words) != 2 || !strings.Contains(words[1], ".") {
			return Contract{}, fmt.Errorf("%q: want `mmio DEVICE.OPERATION`", text)
		}
		return Contract{Kind: MMIO, ID: "mmio " + words[1]}, nil
	case "sync":
		return parseSync(text, words)
	}
	return Contract{}, fmt.Errorf("%q: want `mmio ...` or `sync ...`", text)
}

func parseSync(text string, words []string) (Contract, error) {
	if len(words) < 2 || strings.Contains(words[1], "=") {
		return Contract{}, fmt.Errorf("%q: want `sync OPERATION width=W impl=I [nest=C]`", text)
	}
	fields := map[string]string{}
	for _, w := range words[2:] {
		k, v, ok := strings.Cut(w, "=")
		if !ok || !syncFields[k] || v == "" || fields[k] != "" {
			return Contract{}, fmt.Errorf("%q: %q is not one of width=, impl=, nest=, each given once", text, w)
		}
		fields[k] = v
	}
	if fields["width"] == "" || fields["impl"] == "" {
		return Contract{}, fmt.Errorf("%q: a synchronization operation names its width= and impl=", text)
	}
	id := "sync " + words[1] + " width=" + fields["width"] + " impl=" + fields["impl"]
	if n := fields["nest"]; n != "" {
		id += " nest=" + n
	}
	return Contract{Kind: Sync, ID: id}, nil
}

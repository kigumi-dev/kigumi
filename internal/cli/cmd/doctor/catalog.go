package doctor

import (
	"fmt"
	"os"

	"kigumi/internal/base/errors"
	"kigumi/internal/toolchain/catalog"
)

// catalogField is the std/exp catalog row: where the policy file is, how many
// packages it completed, and whether every std package and layer agreed with
// the resolver.
type catalogField struct {
	field
	FormatVersion int `json:"formatVersion,omitempty"`
	Packages      int `json:"packages,omitempty"`
}

func catalogReport(stdRoot string) (catalogField, []diagnostic) {
	if stdRoot == "" {
		return catalogField{field: field{Status: "missing"}}, nil
	}
	source := catalog.Path(stdRoot)
	c, err := catalog.Generate(stdRoot)
	if err != nil {
		status := "invalid"
		severity := "error"
		if errors.Is(err, os.ErrNotExist) {
			status, severity = "missing", "warning"
		}
		msg := firstLine(err.Error())
		return catalogField{field: field{Value: msg, Source: source, Status: status}},
			[]diagnostic{{Code: "catalog-" + status, Severity: severity, Message: msg, Source: source}}
	}
	return catalogField{
		field:         field{Value: fmt.Sprintf("%d packages", len(c.Packages)), Source: source, Status: "ok"},
		FormatVersion: catalog.FormatVersion,
		Packages:      len(c.Packages),
	}, nil
}

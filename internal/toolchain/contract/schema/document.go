package schema

import (
	"encoding/json"
	"fmt"

	"kigumi/internal/toolchain/version"
)

// Document is `kigumi emit-schema`'s wire form: the typed Schema IR plus the
// two version fields §13.5 makes mandatory before a consumer reads the rest.
// SchemaVersion is the IR's major.minor; CanonicalABIVersion names the WASI
// release whose Canonical ABI the schema's lowering follows.
type Document struct {
	SchemaVersion       string         `json:"schemaVersion"`
	CanonicalABIVersion string         `json:"canonicalAbiVersion"`
	Envelope            Envelope       `json:"envelope"`
	Declarations        []*Declaration `json:"declarations"`
}

// ProjectDocumentJSON renders mod as a versioned binding schema; byte-identical
// input renders byte-identical output.
func ProjectDocumentJSON(mod *Module) ([]byte, error) {
	return json.MarshalIndent(Document{
		SchemaVersion:       schemaVersion(mod.Envelope),
		CanonicalABIVersion: version.CanonicalABIVersion,
		Envelope:            mod.Envelope,
		Declarations:        mod.Declarations,
	}, "", "  ")
}

func schemaVersion(e Envelope) string {
	return fmt.Sprintf("%d.%d", e.Major, e.Minor)
}

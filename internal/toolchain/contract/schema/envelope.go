// Package schema builds the versioned typed Schema IR: the canonical
// compiler/generator boundary that `kigumi schema`'s JSON projects from.
package schema

import (
	"encoding/json"
	"fmt"

	"kigumi/internal/toolchain/version"
)

// Envelope identifies a Schema IR document's format, compiler and query
// scope; a consumer checks Major before reading anything else.
type Envelope struct {
	Format   string   `json:"format"`
	Major    int      `json:"major"`
	Minor    int      `json:"minor"`
	Compiler string   `json:"compiler"`
	Target   string   `json:"target,omitempty"`
	Profile  string   `json:"profile,omitempty"`
	Scope    []string `json:"scope"`
}

const formatName = "kigumi.schema"

func newEnvelope(target, profile string, scope []string) Envelope {
	return Envelope{
		Format:   formatName,
		Major:    version.SchemaIRFormatMajor,
		Minor:    version.SchemaIRFormatMinor,
		Compiler: version.Toolchain,
		Target:   target,
		Profile:  profile,
		Scope:    scope,
	}
}

// DecodeEnvelope reads a document's envelope so a caller can reject an
// unknown major version before parsing the rest; a newer minor is accepted.
func DecodeEnvelope(data []byte) (Envelope, error) {
	var doc struct {
		Envelope
		Nested *Envelope `json:"envelope"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Envelope{}, err
	}
	e := doc.Envelope
	if doc.Nested != nil {
		e = *doc.Nested
	}
	if e.Format != formatName {
		return Envelope{}, fmt.Errorf("schema: not a %s document (format %q)", formatName, e.Format)
	}
	if e.Major != version.SchemaIRFormatMajor {
		return Envelope{}, fmt.Errorf("schema: unsupported format major version %d (reader supports %d)", e.Major, version.SchemaIRFormatMajor)
	}
	return e, nil
}

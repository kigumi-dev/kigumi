package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"kigumi/internal/base/errors"
)

// Path is the policy file inside a std root.
func Path(stdRoot string) string { return filepath.Join(stdRoot, "catalog", "catalog.json") }

// LoadPolicy reads the declared rows. It rejects generated fields so the
// policy file cannot drift from the declarations it is completed from.
func LoadPolicy(stdRoot string) (*Catalog, error) {
	data, err := os.ReadFile(Path(stdRoot))
	if err != nil {
		return nil, errors.WrapErr(err, "read std catalog")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Catalog
	if err := dec.Decode(&c); err != nil {
		return nil, errors.WrapErr(err, "decode std catalog")
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	for _, e := range c.Packages {
		if e.Declarations != (Declarations{}) || len(e.SPI) != 0 {
			return nil, errors.NewErrf("catalog: %s carries generated declarations or SPI; Generate derives those from the sources", e.Path)
		}
	}
	return &c, nil
}

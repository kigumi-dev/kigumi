package catalog

import (
	"encoding/json"

	"kigumi/internal/base/errors"
	"kigumi/internal/toolchain/version"
)

// JSON renders the catalog in the CLI's versioned envelope, so `kigumi
// doctor --catalog` and `kigumi schema --catalog` emit the same document.
func JSON(c *Catalog) ([]byte, error) {
	out, err := json.MarshalIndent(version.Wrap("catalog", FormatVersion, c), "", "  ")
	if err != nil {
		return nil, errors.WrapErr(err, "encode std catalog")
	}
	return append(out, '\n'), nil
}

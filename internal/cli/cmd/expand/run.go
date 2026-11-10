package expand

import (
	"encoding/json"
	"fmt"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/version"
)

func run(io session.IO, opts options) error {
	m, err := opts.session.LoadWhole(opts.target, true, io.Err)
	if err != nil {
		return err
	}
	res, err := driver.Check(m)
	if err != nil {
		return errors.WrapErr(err, "check module")
	}
	if res.HasErrors() {
		fmt.Fprint(io.Err, res.Render())
		return exit.Code(1)
	}
	_, byEntity := schema.Build(m, res, schema.Options{})
	files := schema.GeneratedFiles(res, byEntity)
	if opts.format == "json" {
		data, err := json.MarshalIndent(version.Wrap("expand", version.ExpandFormatVersion, struct {
			Files []schema.GeneratedFile `json:"files"`
		}{Files: files}), "", "  ")
		if err != nil {
			return errors.WrapErr(err, "encode expand output")
		}
		fmt.Fprintln(io.Out, string(data))
		return nil
	}
	for i, f := range files {
		if i > 0 {
			fmt.Fprintln(io.Out)
		}
		fmt.Fprintf(io.Out, "// %s/%s sha256:%s\n", f.Package, f.Name, f.ContentHash)
		for _, src := range f.Sources {
			fmt.Fprintf(io.Out, "//   %s <- %s\n", src.Generator, originName(src))
		}
		fmt.Fprint(io.Out, f.Text)
		if !strings.HasSuffix(f.Text, "\n") {
			fmt.Fprintln(io.Out)
		}
	}
	return nil
}

// originName names the declaration a generated unit came from, or "-" when
// it is outside the query scope (a std type's derived method).
func originName(art schema.GeneratedArtifact) string {
	if art.Origin == nil {
		return "-"
	}
	return art.Origin.ID
}

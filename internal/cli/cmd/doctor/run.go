package doctor

import (
	"encoding/json"
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/catalog"
	"kigumi/internal/toolchain/version"
)

func run(io session.IO, opts options) error {
	if opts.catalog {
		c, err := catalog.Generate(opts.session.StdRoot())
		if err != nil {
			return errors.WrapErr(err, "generate std catalog")
		}
		out, err := catalog.JSON(c)
		if err != nil {
			return err
		}
		_, err = io.Out.Write(out)
		return err
	}
	r := buildReport(opts.session)
	if opts.format == "text" {
		renderText(io.Out, r)
		return nil
	}
	data, err := json.MarshalIndent(version.Wrap("doctor", version.DoctorFormatVersion, r), "", "  ")
	if err != nil {
		return errors.WrapErr(err, "encode doctor report")
	}
	fmt.Fprintln(io.Out, string(data))
	return nil
}

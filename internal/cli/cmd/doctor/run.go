package doctor

import (
	"encoding/json"
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/version"
)

func run(io session.IO, opts options) error {
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

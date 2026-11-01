package parse

import (
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func run(io session.IO, opts options) error {
	src, err := driver.Load(opts.path)
	if err != nil {
		return errors.WrapErr(err, "read source")
	}
	tree, rendered := driver.Parse(src)
	fmt.Fprint(io.Out, tree.Dump())
	if rendered != "" {
		fmt.Fprint(io.Err, rendered)
		return exit.Code(1)
	}
	return nil
}

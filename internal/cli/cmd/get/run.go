package get

import (
	"strings"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func run(io session.IO, opts options) error {
	t, err := openTarget(opts.target, opts.lock)
	if err != nil {
		return err
	}
	if opts.add != "" {
		if err := t.add(io, opts.add, opts.url); err != nil {
			return err
		}
	}
	if err := t.fetch(io, opts); err != nil {
		// Roll back the add so a failed fetch leaves no orphaned Require; scripts have no removal helper yet.
		if opts.add != "" && t.script == "" {
			name, _, _ := strings.Cut(opts.add, "@")
			driver.RemoveRequires(t.dir, map[string]bool{name: true})
		}
		return err
	}
	return nil
}

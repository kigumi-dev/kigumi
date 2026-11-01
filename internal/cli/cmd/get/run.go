package get

import (
	"strings"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
)

func run(io session.IO, opts options) error {
	t, err := openTarget(opts.target, opts.lock)
	if err != nil {
		return err
	}
	git := newGit(io)
	if opts.add != "" {
		if err := t.add(io, git, opts.add, opts.url); err != nil {
			return err
		}
	}
	if err := t.fetch(io, git, opts); err != nil {
		// Roll back the add so a failed fetch leaves no orphaned Require; scripts have no removal helper yet.
		if opts.add != "" && t.script == "" {
			name, _, _ := strings.Cut(opts.add, "@")
			module.RemoveRequires(t.dir, map[string]bool{name: true})
		}
		return err
	}
	return nil
}

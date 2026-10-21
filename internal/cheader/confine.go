package cheader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func confineIncludes(opts Options) error {
	if opts.Root == "" {
		return nil
	}
	run := opts.UserDepRun
	if run == nil {
		run = RunZigUserDeps
	}
	deps, err := run(opts.ZigPath, opts.Target, opts.HeaderPath)
	if err != nil {
		return fmt.Errorf("discovering %s's includes: %w", opts.HeaderPath, err)
	}
	root, err := opts.realPath(opts.Root)
	if err != nil {
		return err
	}
	outside := func(d string) error {
		return fmt.Errorf("%s includes %s, which is outside the module directory %s", opts.HeaderPath, d, opts.Root)
	}
	for _, d := range append([]string{opts.HeaderPath}, deps...) {
		real, err := opts.realPath(d)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if rel, err := filepath.Rel(root, real); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return outside(d)
		}
	}
	return nil
}

func (o Options) realPath(p string) (string, error) {
	if o.Abs != nil {
		abs, err := o.Abs(p)
		if err != nil {
			return "", err
		}
		p = abs
	}
	return filepath.EvalSymlinks(p)
}

package fmt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

func run(io session.IO, opts options) error {
	paths, err := expandDirs(opts.paths)
	if err != nil {
		return err
	}
	failed := false
	for _, path := range paths {
		if err := formatOne(io, path, opts.write, opts.check, len(paths) > 1); err != nil {
			failed = true
			fmt.Fprintln(io.Err, strings.TrimRight(err.Error(), "\n"))
		}
	}
	if failed {
		return exit.Code(1)
	}
	return nil
}

// formatOne formats a single file. Its error is reported by the caller, which goes on to the next file.
func formatOne(io session.IO, path string, write, check, several bool) error {
	src, err := driver.Load(path)
	if err != nil {
		return errors.WrapErr(err, "read source")
	}
	out, err := driver.Format(src)
	if err != nil {
		return err
	}
	switch {
	case check:
		if out != string(src.Src) {
			return fmt.Errorf("%s: not formatted", path)
		}
	case write && path != "-":
		if err := driver.WriteFileAtomic(path, []byte(out), 0o644); err != nil {
			return errors.WrapErrf(err, "write %s", path)
		}
	default:
		if several {
			fmt.Fprintf(io.Out, "==> %s <==\n", path)
		}
		fmt.Fprint(io.Out, out)
	}
	return nil
}

// expandDirs replaces each directory argument with the .kg files under
// it, so `kigumi fmt --check .` covers a module.
func expandDirs(paths []string) ([]string, error) {
	var out []string
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil || !st.IsDir() {
			out = append(out, path)
			continue
		}
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && p != path && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(p, ".kg") && !strings.HasSuffix(p, ".derived.kg") {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, errors.WrapErrf(err, "walk %s", path)
		}
	}
	return out, nil
}

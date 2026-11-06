package vm

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

// mkdirAll creates each prefix of path like the native Files.mkdir: an existing entry is fine only if it opens as a directory.
func (m *Machine) mkdirAll(path string) error {
	if path == "" {
		return syscall.ENOENT
	}
	cur := ""
	for i, part := range strings.Split(path, "/") {
		if i > 0 {
			cur += "/"
		}
		cur += part
		if cur == "" || cur == "/" {
			continue
		}
		err := os.Mkdir(m.resolvePath(cur), 0o755)
		if err == nil {
			continue
		}
		if !errors.Is(err, syscall.EEXIST) {
			return err
		}
		if err := existingDirErr(m.resolvePath(cur)); err != nil {
			return err
		}
	}
	return nil
}

func existingDirErr(path string) error {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return nil
	case err == nil, errors.Is(err, syscall.ENOTDIR):
		return syscall.ENOTDIR
	case errors.Is(err, syscall.ENOENT), errors.Is(err, syscall.ELOOP):
		return syscall.EEXIST
	}
	return nil
}

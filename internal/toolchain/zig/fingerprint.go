package zig

import (
	"fmt"
	"os"
)

// Fingerprint identifies a binary by size and mtime, not its content: a
// cache hit costs one stat call and never invokes it.
func Fingerprint(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat zig at %s: %w", path, err)
	}
	return fmt.Sprintf("%d-%d", st.Size(), st.ModTime().UnixNano()), nil
}

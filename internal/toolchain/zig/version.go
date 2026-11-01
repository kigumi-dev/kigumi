package zig

import (
	"os/exec"
	"strings"
	"sync"
)

var (
	versionMu    sync.Mutex
	versionCache = map[string]string{}
)

// Version runs `zig version`, cached by Fingerprint so a repeated call
// with an unchanged binary never invokes zig again.
func Version(path string) (string, error) {
	fp, err := Fingerprint(path)
	if err != nil {
		return "", err
	}
	versionMu.Lock()
	if v, ok := versionCache[fp]; ok {
		versionMu.Unlock()
		return v, nil
	}
	versionMu.Unlock()
	out, err := exec.Command(path, "version").Output()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	versionMu.Lock()
	versionCache[fp] = v
	versionMu.Unlock()
	return v, nil
}

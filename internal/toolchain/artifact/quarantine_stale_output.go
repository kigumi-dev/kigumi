package artifact

import (
	"os"
	"strconv"
)

// Quarantine moves any pre-existing file at each path aside before a tool step runs, so a no-op success is judged on what it produced, not a previous build's leftover.
func Quarantine(paths []string) (map[string]string, error) {
	aside := map[string]string{}
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		dst := p + ".stale-" + strconv.Itoa(os.Getpid())
		if err := os.Rename(p, dst); err != nil {
			restoreAside(aside)
			return nil, err
		}
		aside[p] = dst
	}
	return aside, nil
}

// restoreAside puts every already-quarantined file back; it never touches
// a path outside aside, since Quarantine has not reached it.
func restoreAside(aside map[string]string) {
	for p, dst := range aside {
		os.Rename(dst, p)
	}
}

// RestoreQuarantined undoes a failed tool run: a quarantined path gets its last good file back, and a path with none has the tool's partial write removed.
func RestoreQuarantined(paths []string, aside map[string]string) {
	restoreAside(aside)
	for _, p := range paths {
		if _, ok := aside[p]; !ok {
			os.Remove(p)
		}
	}
}

// DropQuarantined discards the quarantined copies once the tool
// run has published a valid replacement for every one of them.
func DropQuarantined(aside map[string]string) {
	for _, dst := range aside {
		os.Remove(dst)
	}
}

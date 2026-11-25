//go:build !unix

package vm

// execFailedBeforeRun is always false off POSIX: Windows creates the child
// through CreateProcessA, whose failure to find the program is a spawn error,
// and the remaining targets do not spawn at all.
func execFailedBeforeRun(error) bool {
	return false
}

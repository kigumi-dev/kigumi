package shell

import (
	"strconv"
	"strings"
	"syscall"
)

// glibcText is strerror's wording where Go's errno table departs from glibc's, which std/ffi's Linux table copies.
func glibcText(errno syscall.Errno, text string) string {
	switch {
	case errno == 0:
		return "Success"
	case errno == syscall.ESTALE:
		return "Stale file handle"
	case strings.HasPrefix(text, "errno "):
		return "Unknown error " + strconv.Itoa(int(errno))
	}
	return text
}

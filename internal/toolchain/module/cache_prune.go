package module

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const strayTempAge = time.Hour

type cachedFile struct {
	path  string
	size  int64
	mtime time.Time
}

// PruneCache drops the oldest files under dir's kind directories once they pass limit, down to three quarters of it, and removes torn
// temporary files; it returns how many it dropped.
func PruneCache(dir string, kinds []string, limit int64) int {
	var files []cachedFile
	var total int64
	for _, kind := range kinds {
		filepath.WalkDir(filepath.Join(dir, kind), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			st, err := d.Info()
			if err != nil {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".tmp-") {
				if time.Since(st.ModTime()) > strayTempAge {
					os.Remove(path)
				}
				return nil
			}
			files = append(files, cachedFile{path, st.Size(), st.ModTime()})
			total += st.Size()
			return nil
		})
	}
	if total <= limit {
		return 0
	}
	slices.SortFunc(files, func(a, b cachedFile) int { return a.mtime.Compare(b.mtime) })
	pruned := 0
	for _, f := range files {
		if total <= limit/4*3 {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
			pruned++
		}
	}
	return pruned
}

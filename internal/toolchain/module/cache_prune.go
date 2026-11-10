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
// temporary files. It returns how many files it dropped and how many bytes remain, which is what the caller keeps as the store's
// accounted size instead of walking the directory on every write.
func PruneCache(dir string, kinds []string, limit int64) (pruned int, remaining int64) {
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
		return 0, total
	}
	slices.SortFunc(files, func(a, b cachedFile) int { return a.mtime.Compare(b.mtime) })
	for _, f := range files {
		if total <= limit/4*3 {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
			pruned++
		}
	}
	return pruned, total
}

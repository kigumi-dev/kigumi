package driver

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	cacheLimitBytes = 256 << 20
	pruneEvery      = 128
	strayTempAge    = time.Hour
)

type cachedFile struct {
	path  string
	size  int64
	mtime time.Time
}

// noteWrite counts a store write and prunes on the first one and every pruneEvery-th after, so a long editor session stays bounded as well as a one-shot command.
func (c *packageCache) noteWrite() {
	c.writes++
	if c.writes%pruneEvery == 1 {
		c.prune()
	}
}

// prune drops the oldest records, artifacts and settled results once the store passes its limit, down to three quarters of it, and removes torn temporary files. A removed entry is only a later miss.
func (c *packageCache) prune() {
	limit := c.limit
	if limit == 0 {
		limit = cacheLimitBytes
	}
	var files []cachedFile
	var total int64
	for _, kind := range []string{"pkg", "api", "check", "unit"} {
		filepath.WalkDir(filepath.Join(c.dir, kind), func(path string, d fs.DirEntry, err error) error {
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
		return
	}
	slices.SortFunc(files, func(a, b cachedFile) int { return a.mtime.Compare(b.mtime) })
	for _, f := range files {
		if total <= limit/4*3 {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
			c.stats.Pruned++
		}
	}
}

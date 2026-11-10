package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"kigumi/internal/toolchain/artifact"
	"kigumi/internal/toolchain/buildgraph"
	"kigumi/internal/toolchain/version"
)

// actionCacheDirName is where an output directory keeps tool action results:
// under the build's own output, like zig's cache, so the tool environment
// needs no HOME and nothing outside the module is written.
const actionCacheDirName = ".kigumi-actions"

// actionCacheFormat moves when the stored entry layout changes; an entry of
// another layout is a miss, never a parse error a build reports.
const actionCacheFormat = 1

// actionCacheStats counts tool action traffic for one build: how many steps
// reused a recorded action, how many had to run, and how many could not be
// cached at all because their identity was incomplete.
type actionCacheStats struct {
	Hits        int
	Misses      int
	Stores      int
	Uncacheable int
}

// actionCache stores a tool step's declared outputs under its action key. A
// read that fails to verify is a miss, so a torn or foreign entry only costs
// a recomputation and never becomes a stale artifact.
type actionCache struct {
	dir string
	// stats is how many steps hit, missed, stored or could not be keyed.
	stats actionCacheStats
	// digests memoizes tool and input content digests by path for this build,
	// so several steps over one compiler hash the binary once.
	digests map[string]fileDigest
}

type fileDigest struct {
	size, modNano int64
	digest        string
}

func newActionCache(dir string) *actionCache {
	return &actionCache{dir: dir, digests: map[string]fileDigest{}}
}

// entryDir shards by the key's first two characters, keeping one directory
// from growing a file per action.
func (c *actionCache) entryDir(key string) string {
	return filepath.Join(c.dir, key[:2], key)
}

func (c *actionCache) manifestPath(key string) string {
	return filepath.Join(c.entryDir(key), "entry.json")
}

func (c *actionCache) blobPath(key string, i int) string {
	return filepath.Join(c.entryDir(key), strconv.Itoa(i)+".bin")
}

// actionEntry is one recorded action: the key it belongs to and the digest,
// size and mode of each declared output, so a restored file is verified
// against exactly what the tool produced.
type actionEntry struct {
	Format  int            `json:"format"`
	Key     string         `json:"key"`
	Outputs []actionOutput `json:"outputs"`
}

type actionOutput struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
	Mode   uint32 `json:"mode"`
}

// lookup restores an action's outputs and reports whether every one verified.
// A missing, torn, retargeted or differently keyed entry is a miss; the
// outputs are only written after every blob matched its recorded digest.
func (c *actionCache) lookup(key string, outputs []string) bool {
	b, err := os.ReadFile(c.manifestPath(key))
	if err != nil {
		return false
	}
	var e actionEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return false
	}
	if e.Format != actionCacheFormat || e.Key != key || len(e.Outputs) != len(outputs) {
		return false
	}
	blobs := make([][]byte, len(outputs))
	for i, o := range e.Outputs {
		data, err := os.ReadFile(c.blobPath(key, i))
		if err != nil || int64(len(data)) != o.Size || sumHex(data) != o.Digest {
			return false
		}
		blobs[i] = data
	}
	for i, data := range blobs {
		if err := artifact.WriteOutput(outputs[i], data, os.FileMode(e.Outputs[i].Mode).Perm()); err != nil {
			return false
		}
	}
	return true
}

// store records a successful action's outputs. Each blob is published before
// the manifest, so a crash leaves an entry a later lookup rejects rather than
// a manifest promising bytes that are not there.
func (c *actionCache) store(key string, outputs, names []string) error {
	if len(outputs) != len(names) {
		return fmt.Errorf("action cache: %d outputs but %d names", len(outputs), len(names))
	}
	dir := c.entryDir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	e := actionEntry{Format: actionCacheFormat, Key: key, Outputs: make([]actionOutput, len(outputs))}
	for i, out := range outputs {
		data, err := os.ReadFile(out)
		if err != nil {
			return err
		}
		fi, err := os.Stat(out)
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", out)
		}
		if err := artifact.WriteOutput(c.blobPath(key, i), data, 0o644); err != nil {
			return err
		}
		e.Outputs[i] = actionOutput{Name: names[i], Size: int64(len(data)), Digest: sumHex(data), Mode: uint32(fi.Mode().Perm())}
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return artifact.WriteOutput(c.manifestPath(key), b, 0o644)
}

// digestFile is the content digest of a regular file, memoized by size and
// modification time for this build. ok is false for anything it cannot read
// as one regular file, which keeps the caller's step uncacheable instead of
// guessing an identity.
func (c *actionCache) digestFile(path string) (digest string, ok bool) {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	if d, hit := c.digests[path]; hit && d.size == fi.Size() && d.modNano == fi.ModTime().UnixNano() {
		return d.digest, true
	}
	sum, err := artifact.DigestFile(path)
	if err != nil {
		return "", false
	}
	c.digests[path] = fileDigest{size: fi.Size(), modNano: fi.ModTime().UnixNano(), digest: sum}
	return sum, true
}

// toolAction is the cache identity of one tool step: the tool (with its
// declared version and executable digest), the symbolic argv, the child
// environment and the declared input content hashes. ok is false when any of
// them is unavailable, so the step always runs rather than risk reuse.
func toolAction(cache *actionCache, rg *ResolvedGraph, s buildgraph.ToolStep, program string, argv, env []string, filePath func(int) string, outNames []string) (action artifact.Action, ok bool) {
	action = artifact.Action{Runner: version.Display(), Args: argv, Env: env, Outputs: outNames}
	if s.Kind == "artifact" {
		action.Tool = "artifact:" + rg.Graph.Artifacts[s.Artifact].Name
	} else {
		fw := rg.Graph.Frameworks[s.Framework]
		action.Tool = "framework:" + fw.Name + ":" + s.Tool
		action.Version = fw.Version
	}
	// program is already the executable the launch will use, so the digest
	// names the bytes that actually run.
	digest, ok := cache.digestFile(program)
	if !ok {
		return action, false
	}
	action.Digest = digest
	for _, in := range s.Inputs {
		d, ok := cache.digestFile(filePath(in))
		if !ok {
			return action, false
		}
		action.Inputs = append(action.Inputs, d)
	}
	return action, true
}

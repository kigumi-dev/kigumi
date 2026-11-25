// Package locktest builds lock documents for tests that need to stand in for
// a fetched module without running the resolver or the fetch path.
package locktest

import (
	"crypto/sha256"
	"encoding/hex"

	"kigumi/internal/toolchain/module"
)

// State returns a selected-only lock state for lk. Short test commit ids are
// expanded to a deterministic full sha1 id, because the published format
// stores only full algorithm-tagged commits.
func State(lk module.Lock) (module.LockState, error) {
	entries := make(map[string]module.LockEntry, len(lk.Entries))
	for key, e := range lk.Entries {
		if e.Name == "" {
			e.Name = key
		}
		e.Commit = fullCommit(e.Commit)
		entries[key] = e
	}
	return module.SelectedLockState(inputs(), entries)
}

// Write writes lk as root/mod.lock.kg.
func Write(root string, lk module.Lock) error {
	state, err := State(lk)
	if err != nil {
		return err
	}
	return module.WriteLockState(root, state)
}

// WriteFile writes lk as a script's explicit lock path.
func WriteFile(path string, lk module.Lock) error {
	state, err := State(lk)
	if err != nil {
		return err
	}
	return module.WriteLockStateFile(path, state)
}

func inputs() string {
	return module.LockInputsDigest(module.LockResolverOrderedV1, nil, nil)
}

func fullCommit(commit string) string {
	if _, err := module.ParseCommitID(commit); err == nil {
		return commit
	}
	sum := sha256.Sum256([]byte(commit))
	return "sha1:" + hex.EncodeToString(sum[:20])
}

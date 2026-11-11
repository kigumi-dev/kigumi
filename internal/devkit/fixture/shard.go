package fixture

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Shards returns the subset of ids this test binary runs. KIGUMI_TEST_SHARD="i/n" splits a
// corpus across n test processes, one CI job each, so the corpus wall time is the slowest shard
// rather than the sum. Unset returns every id, which keeps a plain `go test` complete; a
// malformed selector panics so a typo cannot silently run the whole corpus.
//
// Ids are ordered by a cryptographic hash and dealt round-robin, so even a corpus of a few dozen
// entries lands within one of each other across the shards. Hashing each id independently (the
// old contract) let an unlucky name dominate one shard and leave another idle.
func Shards(ids []string) []string {
	i, n, ok := shardSpec()
	if !ok {
		return ids
	}
	type ranked struct {
		pos int
		sum [sha256.Size]byte
	}
	order := make([]ranked, len(ids))
	for pos, id := range ids {
		order[pos] = ranked{pos: pos, sum: sha256.Sum256([]byte(id))}
	}
	slices.SortFunc(order, func(a, b ranked) int {
		if c := bytes.Compare(a.sum[:], b.sum[:]); c != 0 {
			return c
		}
		return cmp.Compare(ids[a.pos], ids[b.pos])
	})
	picked := make([]bool, len(ids))
	for rank, r := range order {
		if rank%n == i {
			picked[r.pos] = true
		}
	}
	out := make([]string, 0, len(ids)/n+1)
	for pos, id := range ids {
		if picked[pos] {
			out = append(out, id)
		}
	}
	return out
}

// ShardPaths splits a corpus path list by fixture id instead of by path: the
// physical group is not part of an id, so moving a fixture between groups must
// not reshuffle the CI shards. Each path is mapped through IDFromPath, Shards
// selects the ids, and the matching paths are returned in their input order.
func ShardPaths(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	ids := make([]string, len(paths))
	for i, p := range paths {
		ids[i] = IDFromPath(p)
	}
	picked := make(map[string]bool, len(ids))
	for _, id := range Shards(ids) {
		picked[id] = true
	}
	out := make([]string, 0, len(picked))
	for i, id := range ids {
		if picked[id] {
			out = append(out, paths[i])
		}
	}
	return out
}

// ShardShare is the share of a whole-corpus minimum that one shard should still find. A
// minimum smaller than the shard count yields 0: the matching entries may all land in another
// shard, so a caller asserting "at least k ran" must treat 0 as no assertion.
func ShardShare(least int) int {
	_, n, ok := shardSpec()
	if !ok {
		return least
	}
	return least / n
}

func shardSpec() (int, int, bool) {
	spec := os.Getenv("KIGUMI_TEST_SHARD")
	if spec == "" {
		return 0, 0, false
	}
	index, count, found := strings.Cut(spec, "/")
	if found {
		i, err1 := strconv.Atoi(index)
		n, err2 := strconv.Atoi(count)
		if err1 == nil && err2 == nil && n > 0 && i >= 0 && i < n {
			return i, n, true
		}
	}
	panic("KIGUMI_TEST_SHARD: want <index>/<count> with 0 <= index < count, got " + strconv.Quote(spec))
}

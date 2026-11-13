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

// ShardPaths splits a corpus path list by fixture id instead of by path and
// balances the byte weight of each shard. File count alone lets two huge
// fixtures land in one shard, where every corpus test recompiles them. The
// assignment is still ID-based (never group-based), so a move keeps its shard.
func ShardPaths(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	shard, count, ok := shardSpec()
	if !ok || count <= 1 {
		return paths
	}
	type item struct {
		pos    int
		id     string
		weight int64
		hash   [sha256.Size]byte
	}
	items := make([]item, len(paths))
	for pos, path := range paths {
		id := IDFromPath(path)
		weight := int64(1)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			weight = info.Size()
		}
		items[pos] = item{pos: pos, id: id, weight: weight, hash: sha256.Sum256([]byte(id))}
	}
	slices.SortFunc(items, func(a, b item) int {
		if a.weight != b.weight {
			return cmp.Compare(b.weight, a.weight)
		}
		if c := bytes.Compare(a.hash[:], b.hash[:]); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})
	load := make([]int64, count)
	assigned := make([]bool, len(paths))
	for _, it := range items {
		pick := 0
		for s := 1; s < count; s++ {
			if load[s] < load[pick] {
				pick = s
			}
		}
		load[pick] += it.weight
		assigned[it.pos] = pick == shard
	}
	out := make([]string, 0, len(paths)/count+1)
	for pos, path := range paths {
		if assigned[pos] {
			out = append(out, path)
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

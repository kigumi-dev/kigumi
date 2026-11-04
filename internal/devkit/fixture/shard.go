package fixture

import (
	"hash/fnv"
	"os"
	"strconv"
	"strings"
)

// Shard reports whether a corpus entry belongs to this test binary. KIGUMI_TEST_SHARD="i/n"
// splits the fixtures across n test processes, one CI job each, so the corpus wall time is the
// slowest shard rather than the sum. Unset or malformed runs everything, which keeps a plain
// `go test` complete; the hash is over the fixture id so every package splits the same way.
func Shard(id string) bool {
	i, n, ok := shardSpec()
	if !ok {
		return true
	}
	h := fnv.New32a()
	h.Write([]byte(id))
	return int(h.Sum32()%uint32(n)) == i
}

// ShardShare is the share of a whole-corpus minimum that one shard should still find, so a
// sanity count keeps its meaning in every shard.
func ShardShare(least int) int {
	_, n, ok := shardSpec()
	if !ok {
		return least
	}
	return max(1, least/n)
}

// Sharded reports whether KIGUMI_TEST_SHARD selected a subset, for checks that count the whole corpus.
func Sharded() bool {
	_, _, ok := shardSpec()
	return ok
}

func shardSpec() (int, int, bool) {
	spec := os.Getenv("KIGUMI_TEST_SHARD")
	if spec == "" {
		return 0, 0, false
	}
	index, count, found := strings.Cut(spec, "/")
	if !found {
		return 0, 0, false
	}
	i, err1 := strconv.Atoi(index)
	n, err2 := strconv.Atoi(count)
	if err1 != nil || err2 != nil || n <= 0 || i < 0 || i >= n {
		return 0, 0, false
	}
	return i, n, true
}

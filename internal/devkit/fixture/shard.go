package fixture

import (
	"hash/fnv"
	"os"
	"strconv"
	"strings"
)

// Shard reports whether a corpus entry belongs to this test binary. KIGUMI_TEST_SHARD="i/n"
// splits the fixtures across n test processes, one CI job each, so the corpus wall time is the
// slowest shard rather than the sum. Unset runs everything, which keeps a plain `go test`
// complete; a malformed selector panics so a typo cannot silently run the whole corpus.
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

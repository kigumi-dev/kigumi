package fixture

import (
	"slices"
	"testing"
)

func fixtureKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "fixture_" + itoa(i) + ".txtar"
	}
	return keys
}

func TestShardsPartitionsKeys(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "0/4")
	keys := fixtureKeys(40)
	seen := map[string]int{}
	for shard := range 4 {
		t.Setenv("KIGUMI_TEST_SHARD", itoa(shard)+"/4")
		for _, key := range Shards(keys) {
			seen[key]++
		}
	}
	if len(seen) != len(keys) {
		t.Fatalf("%d of %d keys never ran", len(keys)-len(seen), len(keys))
	}
	for key, n := range seen {
		if n != 1 {
			t.Errorf("%s ran in %d shards, want 1", key, n)
		}
	}
}

// A corpus of a few dozen entries must still spread within one entry across the shards; the
// per-id hash modulo count this replaced skewed small corpora by nearly 2x.
func TestShardsBalancesSmallCorpora(t *testing.T) {
	for _, size := range []int{7, 13, 51, 109} {
		keys := fixtureKeys(size)
		counts := make([]int, 4)
		for shard := range 4 {
			t.Setenv("KIGUMI_TEST_SHARD", itoa(shard)+"/4")
			counts[shard] = len(Shards(keys))
		}
		least, most := slices.Min(counts), slices.Max(counts)
		if most-least > 1 {
			t.Errorf("corpus of %d split as %v, want every shard within one entry", size, counts)
		}
	}
}

func TestShardsKeepsInputOrder(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "0/2")
	keys := fixtureKeys(10)
	got := Shards(keys)
	i := 0
	for _, key := range keys {
		if i < len(got) && key == got[i] {
			i++
		}
	}
	if i != len(got) {
		t.Fatalf("Shards returned %v, which is not an in-order subset of %v", got, keys)
	}
}

func TestShardShareFloors(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "0/4")
	if got := ShardShare(20); got != 5 {
		t.Errorf("ShardShare(20) = %d, want 5", got)
	}
	if got := ShardShare(2); got != 0 {
		t.Errorf("ShardShare(2) = %d, want 0 (smaller than the shard count)", got)
	}
}

func TestShardsWithoutSpecRunsEverything(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "")
	keys := fixtureKeys(3)
	if got := Shards(keys); !slices.Equal(got, keys) {
		t.Fatalf("an unset spec must run every entry, got %v", got)
	}
}

func TestShardsRejectsMalformedSpec(t *testing.T) {
	for _, spec := range []string{"1", "4/4", "a/b", "0/0", "-1/4", " 0/4"} {
		t.Run(spec, func(t *testing.T) {
			t.Setenv("KIGUMI_TEST_SHARD", spec)
			defer func() {
				if recover() == nil {
					t.Fatalf("Shards accepted %q", spec)
				}
			}()
			Shards([]string{"any.txtar"})
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

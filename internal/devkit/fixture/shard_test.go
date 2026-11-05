package fixture

import (
	"testing"
)

func TestShardPartitionsKeys(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "0/4")
	var keys []string
	for i := 0; i < 40; i++ {
		keys = append(keys, "fixture_"+itoa(i)+".txtar")
	}
	seen := map[string]int{}
	for shard := 0; shard < 4; shard++ {
		t.Setenv("KIGUMI_TEST_SHARD", itoa(shard)+"/4")
		for _, key := range keys {
			if Shard(key) {
				seen[key]++
			}
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

func TestShardShareFloors(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "0/4")
	if got := ShardShare(20); got != 5 {
		t.Errorf("ShardShare(20) = %d, want 5", got)
	}
	if got := ShardShare(2); got != 1 {
		t.Errorf("ShardShare(2) = %d, want 1", got)
	}
}

func TestShardWithoutSpecRunsEverything(t *testing.T) {
	t.Setenv("KIGUMI_TEST_SHARD", "")
	if !Shard("any.txtar") {
		t.Fatal("an unset spec must run every entry")
	}
}

func TestShardRejectsMalformedSpec(t *testing.T) {
	for _, spec := range []string{"1", "4/4", "a/b", "0/0", "-1/4", " 0/4"} {
		t.Run(spec, func(t *testing.T) {
			t.Setenv("KIGUMI_TEST_SHARD", spec)
			defer func() {
				if recover() == nil {
					t.Fatalf("Shard accepted %q", spec)
				}
			}()
			Shard("any.txtar")
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

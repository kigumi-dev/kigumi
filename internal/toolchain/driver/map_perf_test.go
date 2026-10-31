package driver_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const fnvPrime = 1099511628211

func collidingKeys(n, l int, low uint) []string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	mask := uint64(1)<<low - 1
	step := func(s uint64, c byte) uint64 { return ((s ^ uint64(c)) * fnvPrime) & mask }
	start := (uint64(14695981039346656037)) & mask
	target := start
	for range l {
		target = step(target, alphabet[0])
	}
	completions := make([][]int, l+1)
	for j := range completions {
		completions[j] = make([]int, mask+1)
	}
	completions[0][target] = 1
	for j := 1; j <= l; j++ {
		for s := uint64(0); s <= mask; s++ {
			for i := 0; i < len(alphabet); i++ {
				completions[j][s] += completions[j-1][step(s, alphabet[i])]
			}
		}
	}
	var keys []string
	buf := make([]byte, l)
	var walk func(depth int, s uint64)
	walk = func(depth int, s uint64) {
		if len(keys) == n {
			return
		}
		if depth == l {
			keys = append(keys, string(buf))
			return
		}
		for i := 0; i < len(alphabet); i++ {
			next := step(s, alphabet[i])
			if completions[l-depth-1][next] == 0 {
				continue
			}
			buf[depth] = alphabet[i]
			walk(depth+1, next)
		}
	}
	walk(0, start)
	return keys
}

func collidingJSON(t *testing.T, n int) string {
	keys := collidingKeys(n, 5, 14)
	if len(keys) != n {
		t.Fatalf("found %d colliding keys, want %d", len(keys), n)
	}
	var obj strings.Builder
	obj.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			obj.WriteString(",")
		}
		fmt.Fprintf(&obj, "\\\"%s\\\":%d", k, i)
	}
	obj.WriteString("}")
	return fmt.Sprintf("import json from std/json\n\nlet value = json.parse(\"%s\")?\nprint \"parsed\"\n", obj.String())
}

func TestRunCollidingMapKeysStayFast(t *testing.T) {
	if got := runGeneratedVM(t, collidingJSON(t, 8192), 15*time.Second); got != "parsed\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestAotCollidingMapKeysStayFast(t *testing.T) {
	if got := runGeneratedAOT(t, collidingJSON(t, 8192), 3*time.Second); got != "parsed\n" {
		t.Fatalf("stdout %q", got)
	}
}

const removeAllSource = `import json from std/json
import map from std/map

let n = 4000
let mut obj = map.empty[String, Int]()
let mut i = 0
for i < n {
    obj.set("key${i}", i)
    i = i + 1
}
let text = json.encode(obj)?
let back = json.decode[map.Map[String, Int]](text)?
print "round trip ${back.len()} ${back.get(&"key1234") || -1}"
let mut ints = map.empty[Int, Int]()
i = 0
for i < n {
    ints.set(i, i)
    i = i + 1
}
i = 0
for i < n {
    ints.remove(&i)
    i = i + 1
}
print "removed ${ints.len()}"
`

func TestRunMapRemoveAndJSONStayLinear(t *testing.T) {
	if got := runGeneratedVM(t, removeAllSource, 15*time.Second); got != "round trip 4000 1234\nremoved 0\n" {
		t.Fatalf("stdout %q", got)
	}
}

func TestAotMapRemoveAndJSONStayLinear(t *testing.T) {
	if got := runGeneratedAOT(t, removeAllSource, 3*time.Second); got != "round trip 4000 1234\nremoved 0\n" {
		t.Fatalf("stdout %q", got)
	}
}

const drainInOrderSource = `import map from std/map

let n = 150000
let mut m = map.empty[Int, Int]()
let mut i = 0
for i < n {
    m.set(i, i)
    i = i + 1
}
i = 0
let mut sum = 0
for i < n {
    sum = sum + (m.take(&i) || 0)
    i = i + 1
}
print "${m.len()} ${sum}"
`

func TestAotMapDrainInInsertionOrderStaysLinear(t *testing.T) {
	if got := runGeneratedAOT(t, drainInOrderSource, 4*time.Second); got != "0 11249925000\n" {
		t.Fatalf("stdout %q", got)
	}
}

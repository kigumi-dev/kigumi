package vm_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

const secretSanOpsSrc = `import secret from std/secret
import map from std/map
import sort from std/sort
import {Box, Shared} from std/alloc

// sx carries the coarse label classify gives, which tracks no bits.
fn sx(s: u64) -> u64 {
    let t = FixedArray.filled[u64, 4](s)
    secret.ctLookupU64(&t, secret.SecretIndex[4].from(secret.SecretU32.classify(2)))
}

fn br(v: u64) -> u64 {
    if v == 7 { 1 } else { 0 }
}

fn op_xor(s: u64) -> u64 { br(sx(s) ^ 0) }
fn op_or(s: u64) -> u64 { br(sx(s) | 0) }
fn op_and(s: u64) -> u64 { br(sx(s) & 255) }
fn op_shl(s: u64) -> u64 { br(sx(s) << 0) }
fn op_shr(s: u64) -> u64 { br(sx(s) >> 0) }
fn op_not(s: u64) -> u64 { br(~sx(s)) }
fn op_cast(s: u64) -> u64 { br(sx(s).truncate[u8]().toU64()) }

fn shift_count(s: u64) -> u64 {
    let one: u64 = 1
    one << sx(s).truncate[usize]()
}

fn shift_overflow(s: u64) -> u8 {
    sx(s).truncate[u8]() << 1
}

fn map_key(s: u64) -> u64 {
    let mut m = map.empty[u64, u64]()
    m.set(sx(s), 1)
    1
}

fn filter_secret(s: u64) -> usize {
    let x = sx(s)
    Array.of[u64](1, 7, 3).filter((v: &u64) => v == x).len()
}

fn sort_secret(s: u64) -> u64 {
    let ys = sort.stable(&Array.of[u64](sx(s), 2, 9), (a: &u64, b: &u64) => a.compareTo(b))
    ys.len().toU64()
}

fn sort_public_order(s: u64) -> u64 {
    let one: u64 = 1
    let ys = sort.stable(&Array.of[u64](sx(s), 2, 9), (a: &u64, b: &u64) => one.compareTo(&2))
    ys[0]
}

fn filter_public(s: u64) -> u64 {
    Array.of[u64](sx(s), 3).filter((v: &u64) => true)[0]
}

fn box_new(s: u64) -> u64 { Box.new(sx(s)).get() }
fn shared_new(s: u64) -> u64 { Shared.new(sx(s)).get() }

fn insert_at(s: u64) -> u64 {
    let mut xs = Array.of[u64](1, 3)
    xs.insertAt(0, sx(s))
    xs[0]
}

fn array_concat(s: u64) -> u64 { Array.of[u64](sx(s), 3).concat(Array.of[u64](4))[0] }

fn bytes_concat(s: u64) -> u8 {
    let mut buf = ByteBuffer.tryNew(1).unwrap()
    buf.set(0, sx(s).truncate[u8]())
    buf.freeze().concat("zz".toBytes()).get(0) || 0
}

fn valid(r: String!) -> u64 {
    if r is Ok(_) { 1 } else { 0 }
}

fn secret_result(s: u64) -> String! {
    let mut b = ByteBuffer.tryNew(1).unwrap()
    b.set(0, sx(s).wrappingAdd(121).truncate[u8]())
    String.fromBytes(b.freeze())
}

fn cross_param(s: u64) -> u64 { valid(secret_result(s)) }

fn cross_return(s: u64) -> u64 {
    let ok = secret_result(s) is Ok(_)
    if ok { 1 } else { 0 }
}

fn main() {
    _ = op_xor(1)
    _ = op_or(1)
    _ = op_and(1)
    _ = op_shl(1)
    _ = op_shr(1)
    _ = op_not(1)
    _ = op_cast(1)
    _ = shift_count(1)
    _ = shift_overflow(1)
    _ = map_key(1)
    _ = filter_secret(1)
    _ = sort_secret(1)
    _ = sort_public_order(1)
    _ = filter_public(1)
    _ = box_new(1)
    _ = shared_new(1)
    _ = insert_at(1)
    _ = array_concat(1)
    _ = bytes_concat(1)
    _ = cross_param(1)
    _ = cross_return(1)
}
`

// TestSecretSanOperatorsAndCallbacks: operators keep a coarse label; shift counts, map keys, predicates and comparators
// are sinks; a fallible allocation keeps its Result tag public; a Result crossing a call keeps its tag's label.
func TestSecretSanOperatorsAndCallbacks(t *testing.T) {
	prog := secretSanProgram(t, secretSanOpsSrc)
	tests := []struct {
		fn   string
		sink string
	}{
		{"op_xor", "a conditional branch"},
		{"op_or", "a conditional branch"},
		{"op_and", "a conditional branch"},
		{"op_shl", "a conditional branch"},
		{"op_shr", "a conditional branch"},
		{"op_not", "a conditional branch"},
		{"op_cast", "a conditional branch"},
		{"shift_count", "a shift count"},
		{"shift_overflow", "an overflow check"},
		{"map_key", "an index"},
		{"filter_secret", "a conditional branch"},
		{"sort_secret", "a conditional branch"},
		{"sort_public_order", ""},
		{"filter_public", ""},
		{"box_new", ""},
		{"shared_new", ""},
		{"insert_at", ""},
		{"array_concat", ""},
		{"bytes_concat", ""},
		{"cross_param", "a conditional branch"},
		{"cross_return", "a conditional branch"},
	}
	for _, tc := range tests {
		t.Run(tc.fn, func(t *testing.T) {
			sh, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{7}, []bool{false}, nil, nil)
			if tc.sink != "" {
				if err == nil || !strings.Contains(err.Error(), "SecretSan: secret value reaches "+tc.sink) {
					t.Fatalf("%s: err %v, want it to reach %s", tc.fn, err, tc.sink)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tc.fn, err)
			}
			if !sh.Tainted() {
				t.Errorf("%s: the result lost the secret label", tc.fn)
			}
		})
	}
}

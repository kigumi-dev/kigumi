package vm_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
)

const secretSanFlowSrc = `import {atomicLoad, atomicStore, atomicAdd, atomicExchange, atomicCompareExchange, AtomicOrder, UnsafeCell} from std/mem
import mem from std/mem
import {Shared} from std/alloc

type P = {
    pub a u64
}

static mut g: u64 = 0

fn match_option(s: u64) -> u64 {
    match s.toU8() {
        Some(_) -> 1
        None -> 0
    }
}

fn is_option(s: u64) -> u64 {
    if s.toU8() is Some(_) { 1 } else { 0 }
}

fn if_let_result(s: u64) -> u64 {
    if let Ok(_) = String.fromBytes("${s}".toBytes()) { 1 } else { 0 }
}

fn while_is(s: u64) -> u64 {
    let mut n: u64 = 0
    for s.toU8() is Some(_) && n < 1 {
        n = n + 1
    }
    n
}

fn loop_bytes(s: u64) -> u64 {
    let mut n: u64 = 0
    for b in "${s}".toBytes() {
        if b == 55 { n = 1 }
    }
    n
}

fn record_eq(s: u64) -> u64 {
    if P { a: s } == P { a: 7 } { 1 } else { 0 }
}

fn record_hash(s: u64) -> u64 {
    P { a: s }.hash()
}

fn static_store(s: u64) -> u64 {
    unsafe {
        atomicStore(&mut g, s, AtomicOrder.SeqCst)
        atomicLoad(&g, AtomicOrder.SeqCst)
    }
}

fn static_add(s: u64) -> u64 {
    unsafe {
        _ = atomicAdd(&mut g, s, AtomicOrder.SeqCst)
        atomicLoad(&g, AtomicOrder.SeqCst)
    }
}

fn static_exchange(s: u64) -> u64 {
    unsafe {
        _ = atomicExchange(&mut g, s, AtomicOrder.SeqCst)
        atomicLoad(&g, AtomicOrder.SeqCst)
    }
}

fn static_cas(s: u64) -> u64 {
    unsafe {
        _ = atomicCompareExchange(&mut g, s, 3, AtomicOrder.SeqCst)
        atomicLoad(&g, AtomicOrder.SeqCst)
    }
}

fn mmio(s: u64) -> u64 {
    unsafe {
        mem.volatileWriteAt(4096, s)
        mem.volatileReadAt[u64](4096)
    }
}

fn cell_store(s: u64) -> u64 {
    unsafe {
        let c = UnsafeCell.new[u64](0)
        c.atomicStore(s, AtomicOrder.SeqCst)
        c.atomicLoad(AtomicOrder.SeqCst)
    }
}

fn cell_add(s: u64) -> u64 {
    unsafe {
        let c = UnsafeCell.new[u64](0)
        _ = c.atomicAdd(s, AtomicOrder.SeqCst)
        c.atomicLoad(AtomicOrder.SeqCst)
    }
}

fn shared_set(s: u64) -> u64 {
    let c = Shared.new[u64](0)
    c.set(s)
    c.get()
}

fn mmio_address(s: usize) -> u64 {
    unsafe { mem.volatileReadAt[u64](s) }
}

fn sized_alloc(s: usize) -> usize {
    (Array.tryWithCapacity[u64](s) || Array.empty[u64]()).len()
}

fn main() {
    _ = match_option(1)
    _ = is_option(1)
    _ = if_let_result(1)
    _ = while_is(1)
    _ = loop_bytes(1)
    _ = record_eq(1)
    _ = record_hash(1)
    _ = static_store(1)
    _ = static_add(1)
    _ = static_exchange(1)
    _ = static_cas(1)
    _ = mmio(1)
    _ = cell_store(1)
    _ = cell_add(1)
    _ = shared_set(1)
    _ = mmio_address(1)
    _ = sized_alloc(1)
}
`

// TestSecretSanFlows pins the flows outside plain operators: a variant test branches on the scrutinee's tag, a loop
// element, an equality and a hash carry the value's label, and a static, MMIO or cell slot keeps the stored label.
func TestSecretSanFlows(t *testing.T) {
	prog := secretSanProgram(t, secretSanFlowSrc)
	tests := []struct {
		fn   string
		sink string
	}{
		{"match_option", "a conditional branch"},
		{"is_option", "a conditional branch"},
		{"if_let_result", "a conditional branch"},
		{"while_is", "a conditional branch"},
		{"loop_bytes", "a conditional branch"},
		{"record_eq", "a conditional branch"},
		{"record_hash", ""},
		{"static_store", ""},
		{"static_add", ""},
		{"static_exchange", ""},
		{"static_cas", ""},
		{"mmio", ""},
		{"cell_store", ""},
		{"cell_add", ""},
		{"shared_set", ""},
		{"mmio_address", "an address"},
		{"sized_alloc", "an allocation size"},
	}
	for _, tc := range tests {
		t.Run(tc.fn, func(t *testing.T) {
			sh, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{300}, []bool{true}, nil, nil)
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
			if _, err := vm.SecretSanRun(prog, vm.Options{}, tc.fn, []int64{300}, []bool{false}, nil, nil); err != nil {
				t.Errorf("%s with a public input: %v", tc.fn, err)
			}
		})
	}
}

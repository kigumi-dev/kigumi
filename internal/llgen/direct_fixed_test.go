package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

const fixedWorkSrc = `
fn work(seed: Int) -> Int {
    let mut fa = FixedArray.filled[Int, 4](seed)
    fa[1] = fa[0] + 10
    let _ = fa.withMut(2, (v: &mut Int) => { v = v * 3 })
    let mut sum = 0
    for x in fa {
        sum = sum + x
    }
    sum + (fa.len().toInt() || 0) + (fa.get(9) || 0)
}

fn sum3(a: FixedArray[Int, 3]) -> Int { a[0] + a[1] + a[2] }

fn total[T: Copy, const N: usize](a: FixedArray[T, N]) -> usize { a.len() }

fn crossing() -> usize {
    let a = FixedArray.filled[Int, 3](1)
    total(a) + (sum3(a).toUsize() || 0)
}

fn main() -> Unit! {}
`

func TestFixedArrayRunsInPlace(t *testing.T) {
	ir := emitIR(t, fixedWorkSrc)
	if !strings.Contains(ir, `%"kg.arr.std/prelude.FixedArray[Int,4]" = type { [4 x i64] }`) {
		t.Fatalf("FixedArray[Int, 4] is not an inline array:\n%s", ir)
	}
	body := fnBody(t, ir, "main.work")
	checkNoHeap(t, "main.work", body)
	for _, bad := range []string{"@rt_index(", "@rt_setindex(", "@rt_builtin(", `@"kg.box.`, `@"kg.unbox.`, `@"std/prelude.FixedArray`} {
		if strings.Contains(body, bad) {
			t.Errorf("main.work calls %s:\n%s", bad, body)
		}
	}
	if !strings.Contains(body, "icmp uge i64") || !strings.Contains(body, "@rt_panic(") {
		t.Errorf("an index past the end must trap in place:\n%s", body)
	}
}

func TestFixedArrayCrossesIntoAGenericBody(t *testing.T) {
	ir := emitIR(t, fixedWorkSrc)
	sum3 := bodyOf(t, ir, `define i64 @"main.sum3"(ptr %p0)`)
	if !strings.Contains(sum3, `@llvm.memcpy.p0.p0.i64(ptr %l0, ptr %p0, i64 ptrtoint (ptr getelementptr (%"kg.arr.std/prelude.FixedArray[Int,3]"`) || strings.Contains(sum3, "@rt_index(") {
		t.Errorf("sum3 does not copy the 24-byte array in from caller storage:\n%s", sum3)
	}
	body := fnBody(t, ir, "main.crossing")
	if strings.Count(body, `@"kg.box.std/prelude.FixedArray[Int,3]"(`) != 1 {
		t.Errorf("want one box at the generic call:\n%s", body)
	}
	helper := bodyOf(t, ir, `define internal ptr @"kg.box.std/prelude.FixedArray[Int,3]"(`)
	if !strings.Contains(helper, "@rt_array(i64 0") || !strings.Contains(helper, "@rt_array_push(") || !regexp.MustCompile(`icmp ult i64 %v\d+, 3\b`).MatchString(helper) {
		t.Errorf("the boxed form must be an Array of exactly N elements:\n%s", helper)
	}
}

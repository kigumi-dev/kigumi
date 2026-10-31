package llgen_test

import (
	"strings"
	"testing"
)

func TestBorrowedExistentialLendsWithoutAllocatingOrRetaining(t *testing.T) {
	ir := emitIR(t, `
interface Counter {
    fn bump(mut self, by: Int)
    fn get(self) -> Int
}

type Num = {
    pub mut n Int
}

fn Num.bump(mut self, by: Int) {
    self.n = self.n + by
}

fn Num.get(self) -> Int {
    self.n
}

fn read(c: &Counter) -> Int {
    c.get()
}

fn bump(c: &mut Counter) {
    c.bump(1)
}

fn lend(n: &Num) -> Int {
    read(n)
}

fn lendMut(n: &mut Num) {
    bump(n)
}

fn main() -> Void! {
    let mut num = Num { n: 1 }
    lendMut(&mut num)
    print "${lend(&num)}"
}
`)
	for _, fn := range []string{"main.lend", "main.lendMut"} {
		body := fnBody(t, ir, fn)
		checkNoHeap(t, fn, body)
		if !strings.Contains(body, "@rt_box_lent(") {
			t.Errorf("%s does not build its view with rt_box_lent:\n%s", fn, body)
		}
		for _, bad := range []string{"@rt_retain(", "@rt_copy(", "@rt_copy_mut("} {
			if strings.Contains(body, bad) {
				t.Errorf("%s calls %s on a view that owns nothing:\n%s", fn, bad, body)
			}
		}
	}
	read := fnBody(t, ir, "main.read")
	checkNoHeap(t, "main.read", read)
	if strings.Contains(read, "@rt_retain(") {
		t.Errorf("main.read retains the borrowed existential:\n%s", read)
	}
	checkNoHeap(t, "main.bump", fnBody(t, ir, "main.bump"))
}

func TestAsyncBodyBuildsItsViewInTheFutureFrame(t *testing.T) {
	ir := emitIR(t, `
import task from std/task

interface Show {
    fn show(self) -> String
}

type Num = {
    pub n Int
}

fn Num.show(self) -> String {
    "num:${self.n}"
}

async fn inner(v: &Show) -> Int {
    print v.show()
    0
}

async fn outer() -> Int {
    let n = Num { n: 1 }
    await inner(&n)
}

fn main() -> Void! {
    let ex = task.currentThreadExecutor()
    print "${ex.runToCompletion(outer())}"
}
`)
	body := fnBody(t, ir, "main.outer")
	if !strings.Contains(body, "@rt_box_lent(") {
		t.Fatalf("main.outer does not build its view with rt_box_lent:\n%s", body)
	}
	if strings.Contains(body, "alloca [64 x i8]") {
		t.Errorf("main.outer builds its view on the poll function's stack, which a Pending return frees:\n%s", body)
	}
}

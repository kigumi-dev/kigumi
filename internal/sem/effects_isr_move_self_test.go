package sem_test

import (
	"strings"
	"testing"
)

const finishLibSrc = `
pub type Buf = resource {
    pub tag String
}

fn Buf.drop(move self) {
    let s = "${self.tag}"
    _ = s
}

pub noalloc fn buf() -> Buf

pub fn Buf.finish(move self) {}
`

func TestInterruptHandlerMoveSelfReceiverDropsInCallee(t *testing.T) {
	res := checkIsrWithSource(t, finishLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        source.buf().finish()
    }
}
`)
	out := res.Render()
	for _, want := range []string{"E1080", "calls `finish`", "drops `Buf`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a move-self method that lets its receiver fall out of scope must report the drop (missing %q):\n%s", want, out)
		}
	}
}

package llgen_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
)

const asyncFrameSrc = `
import task from std/task

type Point = {
    pub mut x Int
    pub mut y Int
}

async fn never() -> Int {
    await task.spawn(never())
}

async fn walk(n: Int) -> Int {
    let mut p = Point { x: n, y: 0 }
    let label = "walk"
    p.y = await never()
    p.x = p.x + p.y
    print "${label} ${p.x}"
    p.x
}

fn main() -> Unit! {}
`

func TestAsyncFrameKeepsADirectLocalInline(t *testing.T) {
	prog := plannedProgram(t, asyncFrameSrc)
	var walk *mir.Func
	for _, f := range prog.Funcs {
		if f.Name == "main.walk" {
			walk = f
		}
	}
	slot := map[string]int{}
	for i, l := range walk.Locals {
		if l.Ent != 0 && walk.Slot[i] >= 0 {
			slot[l.Name] = walk.Slot[i]
		}
	}
	ir := llgen.Emit(prog)
	checkNoHeap(t, "walk", bodyOf(t, ir, `define ptr @"main.walk"(`))
	drop := bodyOf(t, ir, `define internal void @"main.walk$framedrop"(`)
	at := func(s int) string { return fmt.Sprintf("getelementptr ptr, ptr %%slots, i64 %d\n", s) }
	if !strings.Contains(drop, at(slot["label"])) {
		t.Fatalf("the frame drop does not release the String slot %d:\n%s", slot["label"], drop)
	}
	for s := slot["p"]; s < slot["p"]+2; s++ {
		if strings.Contains(drop, at(s)) {
			t.Fatalf("the frame drop releases the Point's word %d:\n%s", s, drop)
		}
	}
	desc := fmt.Sprintf(`@"main.walk$framedrop", i32 %d,`, walk.Frame)
	if !strings.Contains(ir, desc) {
		t.Fatalf("the descriptor does not carry the %d-word frame MIR laid out", walk.Frame)
	}
}

package llgen_test

import (
	"testing"

	"kigumi/internal/mir"
)

func TestEntropyScratchBufferErasedBeforeFree(t *testing.T) {
	prog := plannedProgram(t, "import entropy from std/entropy\nlet e = host.entropy()\nprint \"${e.bytes(4).unwrap().len()} ${e.u64().unwrap() >= 0}\"\n")
	var body *mir.Func
	for _, f := range prog.Funcs {
		if f.Ent != 0 && prog.R.Entity(f.Ent).Name == "bytes" && prog.R.Fn(f.Ent).Owner != 0 && prog.R.Entity(prog.R.Fn(f.Ent).Owner).Name == "Entropy" {
			body = f
		}
	}
	if body == nil {
		t.Fatal("no lowered Entropy.bytes")
	}
	erased, frees := 0, 0
	for _, blk := range body.Blocks {
		pending := false
		for _, in := range blk.Insts {
			if in.Op != mir.OpCall {
				continue
			}
			switch prog.R.Entity(in.Ent).Name {
			case "secureEraseRaw":
				pending = true
				erased++
			case "free":
				frees++
				if !pending {
					t.Errorf("free of the scratch buffer without an erase before it in the same block")
				}
				pending = false
			}
		}
	}
	if frees < 2 || erased != frees {
		t.Errorf("want an erase for each of at least two frees, got %d erases and %d frees", erased, frees)
	}
}

func TestEntropyU64ErasesItsIntermediateBytes(t *testing.T) {
	prog := plannedProgram(t, "import entropy from std/entropy\nlet e = host.entropy()\nprint \"${e.u64().unwrap() >= 0}\"\n")
	var body *mir.Func
	for _, f := range prog.Funcs {
		if f.Ent != 0 && prog.R.Entity(f.Ent).Name == "u64" && prog.R.Fn(f.Ent).Owner != 0 && prog.R.Entity(prog.R.Fn(f.Ent).Owner).Name == "Entropy" {
			body = f
		}
	}
	if body == nil {
		t.Fatal("no lowered Entropy.u64")
	}
	for _, blk := range body.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpCall && prog.R.Entity(in.Ent).Name == "secureErase" {
				return
			}
		}
	}
	t.Error("Entropy.u64 never erases the bytes it reads before releasing them")
}

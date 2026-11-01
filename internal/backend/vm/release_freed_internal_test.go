package vm

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
)

func TestReleaseAndDropInstructionsCountAFreedUse(t *testing.T) {
	for _, op := range []midir.Opcode{midir.OpRelease, midir.OpDrop} {
		t.Run(op.String(), func(t *testing.T) {
			TrackLive(true)
			defer TrackLive(false)
			m := &Machine{sh: newShared(), opts: Options{Sandbox: true}}
			f := &midir.Func{Locals: make([]midir.Local, 1), Blocks: []midir.Block{{Insts: []midir.Inst{{Op: op, Args: []midir.LocalID{0}}}}}}
			fr := m.newFrame(f, m.codeFor(f))
			v := mkInt(1<<20, 0)
			m.release(v)
			if v.k != kFree {
				t.Fatalf("setup: the object is not freed (kind %v)", v.k)
			}
			before := LiveFreedUses()
			fr.locals[0] = v
			m.exec(fr, &f.Blocks[0].Insts[0], &fr.code.blocks[0][0])
			if got := LiveFreedUses() - before; got != 1 {
				t.Fatalf("freed uses recorded = %d, want 1", got)
			}
		})
	}
}

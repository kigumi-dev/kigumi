package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

const fieldSrc = `
fn pairSum(x: (Int, Int?)) -> Int {
    match x {
        (a, Some(r)) -> a + r
        (a, None) -> a
    }
}
`

func TestVerifyVariantsUnderField(t *testing.T) {
	runRuleCases(t, fieldSrc, false, []ruleCase{
		{"payload under a field on the wrong edge", "main.pairSum", "variants", "which may hold None with 0", func(t *testing.T, p *mir.Program, f *midir.Func) {
			read := -1
			for bi := range f.Blocks {
				for _, in := range f.Blocks[bi].Insts {
					if in.Op == midir.OpPayload {
						read = bi
					}
				}
			}
			for bi := range f.Blocks {
				if term := &f.Blocks[bi].Term; term.Op == midir.TermBranch && int(term.Targets[0]) == read {
					term.Targets[0], term.Targets[1] = term.Targets[1], term.Targets[0]
					return
				}
			}
			t.Fatal("no branch to the payload read")
		}},
		{"payload under a field written after the test", "main.pairSum", "variants", "which may hold None with 0", func(t *testing.T, p *mir.Program, f *midir.Func) {
			for bi := range f.Blocks {
				blk := &f.Blocks[bi]
				for k, in := range blk.Insts {
					if in.Op != midir.OpPayload {
						continue
					}
					set := midir.Inst{Op: midir.OpSetField, Args: []midir.LocalID{f.Params[0], in.Args[0]}, Index: 1}
					blk.Insts = append(blk.Insts[:k:k], append([]midir.Inst{set}, blk.Insts[k:]...)...)
					return
				}
			}
			t.Fatal("no payload read")
		}},
	})
}

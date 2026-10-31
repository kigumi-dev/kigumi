package mir

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
)

// Error is a verifier finding: a rule a function breaks, at one point of its body.
type Error struct {
	Func string
	Rule string
	At   string
	Msg  string
}

func (e *Error) Error() string {
	if e.At == "" {
		return fmt.Sprintf("%s: %s: %s", e.Func, e.Rule, e.Msg)
	}
	return fmt.Sprintf("%s: %s: %s: %s", e.Func, e.Rule, e.At, e.Msg)
}

func (p *verifier) check() error {
	fs := p.allFuncs()
	p.pruneHistory(fs)
	for _, f := range fs {
		if err := verifyStructure(f); err != nil {
			err.Func = f.Name
			return err
		}
	}
	if err := p.verifyEntry(); err != nil {
		return err
	}
	for _, f := range fs {
		if err := p.verifyFunc(f); err != nil {
			return err
		}
	}
	return nil
}

func (p *Program) allFuncs() []*midir.Func {
	fs := append([]*midir.Func{}, p.Funcs...)
	for _, cf := range p.Comptime {
		fs = append(fs, cf.Func)
	}
	return append(fs, p.Entries...)
}

func (p *verifier) verifyFunc(f *midir.Func) error {
	defer func() { delete(p.bodies, f) }()
	if err := runRules(f, []func(*midir.Func) *Error{verifyShape, verifyAsync, p.verifyParams, p.verifyEntities, p.verifyTypes, p.verifyStdOpTags, p.verifyPrimCalls, p.verifyCalls, p.verifyAggregates}); err != nil {
		return err
	}
	// Definitions and the flow analyses read only this function and the checked
	// tree, so a body that passed them before and has not changed is not walked again.
	hash := flowHash(f)
	memo, known := p.verified[f]
	changed := !known || memo.hash != hash
	if !changed {
		p.body(f).weakDefs = memo.weak
	}
	if changed {
		if err := p.verifyDefs(f); err != nil {
			err.Func = f.Name
			return err
		}
	}
	if err := runRules(f, []func(*midir.Func) *Error{p.verifyOwnership, p.verifyCellCaptures, p.verifyCellOps, p.verifyCellValues, p.verifyBuiltins, p.verifyDropOwned, p.verifyReturnOwned}); err != nil {
		return err
	}
	if changed {
		if err := runRules(f, []func(*midir.Func) *Error{p.verifyRefs, p.verifyVariants}); err != nil {
			return err
		}
	}
	if err := runRules(f, []func(*midir.Func) *Error{p.verifyDirect, p.verifyDirectSig, p.verifyBoxedEntry, p.verifyDirectForms, p.verifyCrossLocals, p.verifyCounted, p.verifyFixedBoxes, p.verifyAllocClaim}); err != nil {
		return err
	}
	p.rememberFlow(f, hash)
	return nil
}

func runRules(f *midir.Func, rules []func(*midir.Func) *Error) error {
	for _, rule := range rules {
		if err := rule(f); err != nil {
			err.Func = f.Name
			return err
		}
	}
	return nil
}

func at(bid midir.BlockID, i int) string { return fmt.Sprintf("b%d/%d", bid, i) }

func fail(rule string, bid midir.BlockID, i int, format string, args ...any) *Error {
	return &Error{Rule: rule, At: at(bid, i), Msg: fmt.Sprintf(format, args...)}
}

// verifyStructure checks that every reference points at a local or a
// block that exists and that every block ends in exactly one terminator.
func verifyStructure(f *midir.Func) *Error {
	nb, nl := len(f.Blocks), len(f.Locals)
	for _, prm := range f.Params {
		if int(prm) >= nl {
			return &Error{Rule: "structure", Msg: fmt.Sprintf("parameter %d out of range", prm)}
		}
	}
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			if int(in.Dst) >= nl {
				return fail("structure", bid, i, "destination %d out of range", in.Dst)
			}
			for _, a := range in.Args {
				if int(a) >= nl {
					return fail("structure", bid, i, "argument %d out of range", a)
				}
			}
		}
		term := len(blk.Insts)
		switch blk.Term.Op {
		case midir.TermNone:
			return fail("structure", bid, term, "missing terminator")
		case midir.TermJump:
			if len(blk.Term.Targets) != 1 || len(blk.Term.Args) != 0 {
				return fail("structure", bid, term, "jump takes one target and no argument")
			}
		case midir.TermBranch:
			if len(blk.Term.Targets) != 2 || len(blk.Term.Args) != 1 {
				return fail("structure", bid, term, "branch takes two targets and one argument")
			}
		case midir.TermReturn:
			if len(blk.Term.Targets) != 0 || len(blk.Term.Args) != 1 {
				return fail("structure", bid, term, "return takes one argument and no target")
			}
		case midir.TermUnreachable:
			if len(blk.Term.Targets) != 0 || len(blk.Term.Args) != 0 {
				return fail("structure", bid, term, "unreachable takes nothing")
			}
		default:
			return fail("structure", bid, term, "unknown terminator %d", blk.Term.Op)
		}
		for _, t := range blk.Term.Targets {
			if int(t) >= nb {
				return fail("structure", bid, term, "target b%d out of range", t)
			}
		}
		for _, a := range blk.Term.Args {
			if int(a) >= nl {
				return fail("structure", bid, term, "terminator argument %d out of range", a)
			}
		}
	}
	return nil
}

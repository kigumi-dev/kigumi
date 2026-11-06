package vm

import (
	"fmt"
	"math/big"
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// The step, depth and memory limits bound a sandboxed comptime evaluation:
// they fail a non-terminating, too deeply recursive or too memory-hungry
// block with a diagnostic instead of hanging the compiler, overflowing the
// Go stack or exhausting host RAM. Steps and bytes rather than wall clock
// or live RSS, so the same source fails the same way on every machine.
// examples/comptime needs ~28M steps for its naive fib(30); its string
// table is under 1KiB, so comptimeMemoryLimit leaves that fixture ample
// headroom while still catching runaway growth (alloc_budget.go) quickly.
const (
	comptimeStepLimit   = 200_000_000
	comptimeDepthLimit  = 3_000
	comptimeMemoryLimit = 64 * 1024 * 1024
)

// hostEffect marks the primitives a sandbox refuses: anything that reaches
// the file system, processes, the network, the clock, entropy, foreign
// memory or the dynamic loader.
func hostEffect(key string) bool {
	for _, p := range []string{"fs.", "os.", "net.", "shell.", "dl.", "ffi."} {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	switch key {
	case "time.now", "time.sleep", "time.monotonic", "random/clock.fromClock":
		return true
	case "task.BlockingPool.new", "task.BlockingPool.run":
		return true
	}
	return false
}

type ComptimeFailure struct {
	Msg   string
	Site  mir.Site
	Limit string
}

func (e *ComptimeFailure) Error() string { return e.Msg }

// EvalComptime runs one comptime block in the sandbox and returns the
// literal it produced; output the block printed is discarded.
func EvalComptime(prog *mir.Program, cf *mir.ComptimeFunc) (lit sem.Literal, err error) {
	lit, _, err = evalComptime(prog, cf)
	return lit, err
}

func evalComptime(prog *mir.Program, cf *mir.ComptimeFunc) (lit sem.Literal, printed int, err error) {
	if err := checkComptimeLiterals(prog, cf.Func); err != nil {
		return lit, 0, err
	}
	var out byteCounter
	m := newMachine(prog, Options{Sandbox: true, StepLimit: comptimeStepLimit, DepthLimit: comptimeDepthLimit, MemoryLimit: comptimeMemoryLimit}, &out, &out)
	var v *obj
	cerr := m.catchAtUser(func() { v = deref(m.call(cf.Func, nil)) })
	if cerr != nil {
		if p, ok := cerr.(*Panic); ok {
			return lit, int(out), &ComptimeFailure{Msg: p.Msg, Site: p.Site, Limit: p.Limit}
		}
		return lit, int(out), cerr
	}
	lit, err = m.literalOf(v, cf.Type)
	return lit, int(out), err
}

type byteCounter int

func (c *byteCounter) Write(p []byte) (int, error) {
	*c += byteCounter(len(p))
	return len(p), nil
}

func (m *Machine) literalOf(v *obj, t sem.TypeID) (sem.Literal, error) {
	v = deref(v)
	switch v.k {
	case kInt:
		if is128(v.nkind()) {
			return sem.Literal{Kind: sem.LitInt, Int: bigOfWide(widen(v), isSigned(v.nkind()))}, nil
		}
		return sem.Literal{Kind: sem.LitInt, Int: big.NewInt(v.i)}, nil
	case kFloat:
		return sem.Literal{Kind: sem.LitFloat, Float: big.NewFloat(v.fl())}, nil
	case kBool:
		return sem.Literal{Kind: sem.LitBool, Bool: v.b}, nil
	case kChar:
		return sem.Literal{Kind: sem.LitChar, Char: v.ch()}, nil
	case kStr:
		return sem.Literal{Kind: sem.LitString, Str: string(v.str())}, nil
	case kBytes:
		return sem.Literal{Kind: sem.LitBytes, Str: string(v.str())}, nil
	case kRecord:
		fields := m.r.TypeDecl(m.typeEnt(t)).Fields
		if len(fields) != len(v.fields) {
			break
		}
		out := sem.Literal{Kind: sem.LitRecord}
		for i, f := range fields {
			fl, err := m.literalOf(v.fields[i], m.r.Entity(f).Type)
			if err != nil {
				return fl, err
			}
			out.Fields = append(out.Fields, fl)
		}
		return out, nil
	}
	return sem.Literal{}, fmt.Errorf("comptime value %s cannot be embedded as a literal", m.display(v))
}

// ComptimeError is a comptime block that failed to evaluate, or one that printed output nobody can see.
type ComptimeError struct {
	Site sem.ComptimeSite
	Err  error
	// Printed is the number of bytes a block that evaluated fine wrote to stdout or stderr.
	Printed int
}

// FoldComptime evaluates every comptime block of the program, in the order
// the checker met them (inner blocks first), and folds each result into
// the code that uses it. The failures come back per block.
func FoldComptime(prog *mir.Program) []ComptimeError {
	var errs []ComptimeError
	for _, cf := range prog.Comptime {
		lit, printed, err := evalComptime(prog, cf)
		if err != nil {
			errs = append(errs, ComptimeError{Site: cf.Site, Err: err})
			continue
		}
		if printed > 0 {
			errs = append(errs, ComptimeError{Site: cf.Site, Printed: printed})
		}
		prog.Fold(cf, lit)
	}
	return errs
}

func (m *Machine) userSite(site mir.Site) mir.Site {
	for i := len(m.frames) - 1; i >= 0; i-- {
		fr := m.frames[i]
		if fr.f == nil || fr.f.Ent != 0 && m.r.Packages[m.r.Entity(fr.f.Ent).Pkg].Std {
			continue
		}
		if i == len(m.frames)-1 {
			return site
		}
		if fr.blk < midir.BlockID(len(fr.f.Blocks)) && fr.pc < len(fr.f.Blocks[fr.blk].Insts) {
			return m.p.SiteAt(fr.f, fr.f.Blocks[fr.blk].Insts[fr.pc].Node)
		}
		return mir.Site{}
	}
	return mir.Site{}
}

// catchAtUser moves a panic's site from std to the user line that led to it.
func (m *Machine) catchAtUser(fn func()) error {
	return m.catch(func() {
		defer func() {
			r := recover()
			if p, ok := r.(*Panic); ok {
				p.Site = m.userSite(p.Site)
			}
			if r != nil {
				panic(r)
			}
		}()
		fn()
	})
}

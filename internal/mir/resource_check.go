package mir

import (
	"fmt"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/sem"
)

// ResourceOptions configures the strict stack-budget gate. Strict false
// leaves every chain report-only: the hosted-growable default.
type ResourceOptions struct {
	Strict bool
	// BudgetKiB is the available stack capacity a bounded root must fit
	// under; 0 only requires boundedness, no numeric budget.
	BudgetKiB int
}

// CheckResourceBudget gates every stack root under a strict profile: each
// root's Chain must be Bounded, and within BudgetKiB when one is set.
func CheckResourceBudget(prog *Program, resources map[sem.EntityID]*ResourceSummary, opts ResourceOptions) {
	if !opts.Strict {
		return
	}
	for _, ent := range StackRoots(prog.R) {
		f := prog.ByEnt[ent]
		s := resources[ent]
		if f == nil || s == nil {
			continue
		}
		reportRootViolation(prog, f, s.Chain, opts.BudgetKiB)
	}
}

// StackRoots is the process entry plus every C-ABI export the program
// itself declares; std's own export(C) shims stay out, same as MainFn.
func StackRoots(r *sem.Result) []sem.EntityID {
	var out []sem.EntityID
	if m := r.MainFn(); m != 0 {
		out = append(out, m)
	}
	for id := 1; id < len(r.Entities); id++ {
		ent := sem.EntityID(id)
		e := r.Entity(ent)
		if e.Kind != sem.EntFn || r.Fn(ent).Export == "" {
			continue
		}
		if p := r.Packages[e.Pkg]; p.Std || p.Module != "" {
			continue
		}
		out = append(out, ent)
	}
	return out
}

func reportRootViolation(prog *Program, f *Func, chain StackBound, budgetKiB int) {
	name := prog.R.Entity(f.Ent).Name
	switch chain.Kind {
	case Unbounded:
		emitResourceDiag(prog, f, "stack-chain-unbounded", name, pathDetail(chain))
	case Unknown:
		emitResourceDiag(prog, f, "stack-chain-unknown", name, pathDetail(chain))
	case Bounded:
		if budgetKiB <= 0 {
			return
		}
		limit := uint64(budgetKiB) * 1024
		if chain.Bytes > limit {
			emitResourceDiag(prog, f, "stack-budget-exceeded", name, fmt.Sprint(chain.Bytes), fmt.Sprint(limit))
		}
	}
}

func pathDetail(b StackBound) string {
	if len(b.Path) == 0 {
		return b.Reason
	}
	return b.Reason + " (via " + strings.Join(b.Path, " -> ") + ")"
}

func emitResourceDiag(prog *Program, f *Func, codeName string, args ...any) {
	c, ok := sem.CodeByName(codeName)
	if !ok {
		return
	}
	msg := sem.FillTemplate(c.Template, args...)
	if c.Help != "" {
		msg += "; " + c.Help
	}
	t := prog.R.Tree(f.File)
	node := prog.R.Entity(f.Ent).Node
	f.Diags = append(f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(node)), Msg: msg, Code: c.Num})
}

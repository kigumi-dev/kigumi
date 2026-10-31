package resource

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

type Options struct {
	Strict    bool
	BudgetKiB int
}

type budgetCheck struct {
	r     *sem.Result
	diags []midir.FileDiag
}

func CheckBudget(r *sem.Result, bodies map[sem.EntityID]*midir.Func, resources map[sem.EntityID]*Summary, opts Options) []midir.FileDiag {
	if !opts.Strict {
		return nil
	}
	check := &budgetCheck{r: r}
	for _, ent := range StackRoots(r) {
		f, s := bodies[ent], resources[ent]
		if f != nil && s != nil {
			check.reportRootViolation(f, s.Chain, opts.BudgetKiB)
		}
	}
	return check.diags
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

func (c *budgetCheck) reportRootViolation(f *midir.Func, chain StackBound, budgetKiB int) {
	name := c.r.Entity(f.Ent).Name
	switch chain.Kind {
	case Unbounded:
		c.emitResourceDiag(f, "stack-chain-unbounded", name, pathDetail(chain))
	case Unknown:
		c.emitResourceDiag(f, "stack-chain-unknown", name, pathDetail(chain))
	case Bounded:
		if budgetKiB <= 0 {
			return
		}
		limit := uint64(budgetKiB) * 1024
		if chain.Bytes > limit {
			c.emitResourceDiag(f, "stack-budget-exceeded", name, fmt.Sprint(chain.Bytes), fmt.Sprint(limit))
		}
	}
}

func pathDetail(b StackBound) string {
	if len(b.Path) == 0 {
		return b.Reason
	}
	return b.Reason + " (via " + strings.Join(b.Path, " -> ") + ")"
}

func (c *budgetCheck) emitResourceDiag(f *midir.Func, codeName string, args ...any) {
	code, ok := sem.CodeByName(codeName)
	if !ok {
		return
	}
	msg := sem.FillTemplate(code.Template, args...)
	if code.Help != "" {
		msg += "; " + code.Help
	}
	t := c.r.Tree(f.File)
	node := c.r.Entity(f.Ent).Node
	c.diags = append(c.diags, midir.FileDiag{File: f.File, Diag: diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(node)), Msg: msg, Code: code.Num}})
}

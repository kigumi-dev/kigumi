package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// leafRule is how a node yields its value: as itself, not at all (it diverges), through the tail,
// branch, body or `break` children that produce it, or never (it is no value expression).
type leafRule uint8

const (
	leafSelf leafRule = iota + 1
	leafNone
	leafParen
	leafTail
	leafIf
	leafIfLet
	leafMatch
	leafBody
	leafLoop
	leafOpaque
)

// leafKinds lists every node kind exactly once; TestLeafRulesCoverEveryKind keeps it that way.
var leafKinds = map[leafRule][]syntax.NodeKind{
	leafParen: {syntax.Paren},
	leafTail:  {syntax.Block},
	leafIf:    {syntax.IfExpr},
	leafIfLet: {syntax.IfLetExpr},
	leafMatch: {syntax.MatchExpr},
	leafBody:  {syntax.UnsafeExpr, syntax.AllocatorExpr, syntax.ContextExpr, syntax.ContractExpr},
	leafLoop:  {syntax.ForExpr},
	leafNone:  {syntax.ReturnExpr, syntax.FailExpr, syntax.BreakExpr, syntax.ContinueExpr},
	leafSelf: {
		syntax.Ident, syntax.IntLit, syntax.FloatLit, syntax.CharLit, syntax.BoolLit, syntax.StringLit, syntax.ByteStringLit,
		syntax.ShellLit, syntax.Unary, syntax.Binary, syntax.IsExpr, syntax.CallExpr, syntax.WsCallExpr, syntax.BracketExpr,
		syntax.MemberExpr, syntax.OptMemberExpr, syntax.TryExpr, syntax.RecordLit, syntax.TupleLit, syntax.UnitLit,
		syntax.Lambda, syntax.AwaitExpr, syntax.ComptimeExpr, syntax.BorrowExpr, syntax.MoveExpr, syntax.AsmExpr,
	},
	leafOpaque: {
		syntax.NoneKind, syntax.FieldInit, syntax.Spread, syntax.NamedArg, syntax.MatchArm, syntax.AsmTemplate,
		syntax.AsmOperand, syntax.AsmClobber, syntax.AsmOptions, syntax.AsmAbi, syntax.PatWildcard, syntax.PatBind,
		syntax.PatLit, syntax.PatCtor, syntax.PatRecord, syntax.PatOr, syntax.PatField, syntax.PatTuple, syntax.PatRange,
		syntax.ShellPlan, syntax.ShellCmd, syntax.ShellWord, syntax.ShellText, syntax.ShellInterp, syntax.ShellRedirect,
		syntax.DefaultArg, syntax.File, syntax.FnDecl, syntax.Param, syntax.GenericParam, syntax.TypeDecl,
		syntax.RecordBody, syntax.ResourceBody, syntax.AdtBody, syntax.AliasBody, syntax.DefinedBody, syntax.Field,
		syntax.Variant, syntax.VariantField, syntax.Metadata, syntax.MetaItem, syntax.InterfaceDecl, syntax.ConstDecl,
		syntax.StaticDecl, syntax.ContextKeyDecl, syntax.ImportDecl, syntax.ImportAlias, syntax.TestDecl, syntax.AbiBlock,
		syntax.ContractBlock, syntax.InstantiateDecl, syntax.LayoutClause, syntax.Attr, syntax.UnsafeGuarantee,
		syntax.UGRule, syntax.UGRequirement, syntax.Visibility, syntax.LetStmt, syntax.ExprStmt, syntax.AssignStmt,
		syntax.DeferStmt, syntax.DeferFailStmt, syntax.List, syntax.TypePath, syntax.TypeLifetime, syntax.TypeOptional,
		syntax.TypeResult, syntax.TypeFn, syntax.TypePtr, syntax.TypeRef, syntax.TypeTuple, syntax.Path,
	},
}

var leafRules = func() map[syntax.NodeKind]leafRule {
	out := map[syntax.NodeKind]leafRule{}
	for rule, kinds := range leafKinds {
		for _, k := range kinds {
			out[k] = rule
		}
	}
	return out
}()

func (c *checker) tailLeaves(n syntax.NodeID) []syntax.NodeID {
	if n == 0 {
		return nil
	}
	v := c.t.View(n)
	switch leafRules[c.t.Kind(n)] {
	case leafNone:
		return nil
	case leafParen:
		return c.tailLeaves(v.AsParen().Inner().ID())
	case leafTail:
		if tail := v.AsBlock().Tail(); tail.Valid() {
			return c.tailLeaves(tail.ID())
		}
		return nil
	case leafIf:
		s := ifExpr(c.t, n)
		return append(c.tailLeaves(s.Then), c.tailLeaves(s.Else)...)
	case leafIfLet:
		s := ifLet(c.t, n)
		return append(c.tailLeaves(s.Then), c.tailLeaves(s.Else)...)
	case leafMatch:
		var out []syntax.NodeID
		for arm := range v.AsMatch().Arms().IDs() {
			out = append(out, c.tailLeaves(matchArm(c.t, arm).Body)...)
		}
		return out
	case leafBody:
		return c.tailLeaves(c.leafBody(v))
	case leafLoop:
		var out []syntax.NodeID
		for _, b := range c.loopBreaks(forExpr(c.t, n).Body) {
			out = append(out, c.tailLeaves(b)...)
		}
		return out
	}
	return []syntax.NodeID{n}
}

// loopBreaks are the `break` values under n that leave the loop n belongs to: a nested loop's head
// and a `||` handler arm still break out to it, while a nested loop's body and a closure do not.
func (c *checker) loopBreaks(n syntax.NodeID) []syntax.NodeID {
	v := c.t.View(n)
	switch c.t.Kind(n) {
	case syntax.NoneKind, syntax.Lambda:
		return nil
	case syntax.ForExpr:
		return c.loopBreaks(forExpr(c.t, n).Head)
	case syntax.BreakExpr:
		if val := v.AsBreak().Value().ID(); val != 0 {
			return []syntax.NodeID{val}
		}
		return nil
	case syntax.Binary:
		if hit, arm, ok := c.orHandler(n); ok {
			return append(c.loopBreaks(hit), c.loopBreaks(arm)...)
		}
	}
	var out []syntax.NodeID
	for ch := range v.Children() {
		out = append(out, c.loopBreaks(ch.ID())...)
	}
	return out
}

// orHandler splits `r || (e) => arm` into r and the arm's body, which runs in place rather than as a closure.
func (c *checker) orHandler(n syntax.NodeID) (syntax.NodeID, syntax.NodeID, bool) {
	if c.t.Kind(n) != syntax.Binary {
		return 0, 0, false
	}
	b := c.t.View(n).AsBinary()
	if b.Op().Kind() != token.OrOr || b.Right().Kind() != syntax.Lambda {
		return 0, 0, false
	}
	return b.Left().ID(), b.Right().AsLambda().Body().ID(), true
}

func (c *checker) leafBody(v syntax.View) syntax.NodeID {
	switch v.Kind() {
	case syntax.UnsafeExpr:
		return v.AsUnsafe().Body().ID()
	case syntax.AllocatorExpr:
		return v.AsAllocator().Body().ID()
	case syntax.ContextExpr:
		return v.AsContext().Body().ID()
	}
	return v.AsContract().Body().ID()
}

// passThrough is the value a node hands on as itself: through parentheses, a block's tail and an
// unsafe / allocator / context / contract body, unlike a join, which copies each arm into a temporary.
func (c *checker) passThrough(n syntax.NodeID) syntax.NodeID {
	for {
		n = c.t.Unparen(n)
		switch leafRules[c.t.Kind(n)] {
		case leafBody:
			n = c.leafBody(c.t.View(n))
		case leafTail:
			tail := c.t.View(n).AsBlock().Tail()
			if !tail.Valid() {
				return n
			}
			n = tail.ID()
		default:
			return n
		}
	}
}

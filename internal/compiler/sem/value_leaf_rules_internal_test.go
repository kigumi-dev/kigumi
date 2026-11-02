package sem

import (
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// leafExamples holds, for every kind a value can be or open through, a body whose tail is that kind.
var leafExamples = map[syntax.NodeKind]string{
	syntax.Ident:         "x",
	syntax.IntLit:        "1",
	syntax.FloatLit:      "1.5",
	syntax.CharLit:       "'a'",
	syntax.BoolLit:       "true",
	syntax.StringLit:     `"s"`,
	syntax.ByteStringLit: `b"s"`,
	syntax.ShellLit:      `$"echo hi"`,
	syntax.Paren:         "(x)",
	syntax.Unary:         "-x",
	syntax.Binary:        "x + 1",
	syntax.IsExpr:        "x is Some(_)",
	syntax.CallExpr:      "f(x)",
	syntax.WsCallExpr:    "print x",
	syntax.BracketExpr:   "xs[0]",
	syntax.MemberExpr:    "x.y",
	syntax.OptMemberExpr: "x?.y",
	syntax.TryExpr:       "x?",
	syntax.RecordLit:     "P { a: x }",
	syntax.TupleLit:      "(x, 1)",
	syntax.UnitLit:       "()",
	syntax.Lambda:        "(a) => a",
	syntax.AwaitExpr:     "await t",
	syntax.ComptimeExpr:  "comptime { 1 }",
	syntax.BorrowExpr:    "&x",
	syntax.MoveExpr:      "move x.y",
	syntax.AsmExpr:       "asm {\n        \"nop\"\n    }",
	syntax.Block:         "{\n        x\n    }",
	syntax.IfExpr:        "if c { x } else { y }",
	syntax.IfLetExpr:     "if let Some(v) = o { v } else { y }",
	syntax.MatchExpr:     "match k {\n        0 -> x\n        _ -> y\n    }",
	syntax.ForExpr:       "for {\n        break x\n    }",
	syntax.UnsafeExpr:    "unsafe { x }",
	syntax.AllocatorExpr: "allocator a.scope() { x }",
	syntax.ContextExpr:   "context (E { k: 1 }) { x }",
	syntax.ContractExpr:  "pure { x }",
}

// TestLeafRulesCoverEveryKind: every node kind has exactly one leaf rule, and every kind a value can
// be or open through has an example that parses to it, so the table and the examples stay in step.
func TestLeafRulesCoverEveryKind(t *testing.T) {
	seen := map[syntax.NodeKind]int{}
	for _, kinds := range leafKinds {
		for _, k := range kinds {
			seen[k]++
		}
	}
	for k := syntax.NodeKind(0); k.Valid(); k++ {
		if seen[k] != 1 {
			t.Errorf("node kind %s has %d leaf rules, want 1", k, seen[k])
		}
	}
	for rule, kinds := range leafKinds {
		if rule == leafNone || rule == leafOpaque {
			continue
		}
		for _, k := range kinds {
			src, ok := leafExamples[k]
			if !ok {
				t.Errorf("node kind %s has no leaf example", k)
				continue
			}
			checkLeafExample(t, k, rule, src)
		}
	}
}

func checkLeafExample(t *testing.T, k syntax.NodeKind, rule leafRule, src string) {
	t.Helper()
	tree := syntax.Parse(token.NewFile("leaf.kg", []byte("fn probe() {\n    "+src+"\n}\n")))
	if tree.HasErrors() {
		t.Errorf("%s example does not parse:\n%s", k, src)
		return
	}
	fn := tree.List(tree.Root).ItemID(0)
	tail := tree.View(fn).AsFnDecl().Body().AsBlock().Tail()
	if !tail.Valid() || tail.Kind() != k {
		t.Errorf("%s example's tail is %v, want %s", k, tail.Kind(), k)
		return
	}
	c := &checker{t: tree}
	leaves := c.tailLeaves(tail.ID())
	switch {
	case rule == leafSelf && (len(leaves) != 1 || leaves[0] != tail.ID()):
		t.Errorf("%s is its own leaf, got %d leaves", k, len(leaves))
	case rule != leafSelf && (len(leaves) == 0 || leaves[0] == tail.ID()):
		t.Errorf("%s opens to the value it passes on, got %d leaves", k, len(leaves))
	}
	for _, l := range leaves {
		if leafRules[tree.Kind(l)] != leafSelf {
			t.Errorf("%s yields a %s leaf, which is not a value", k, tree.Kind(l))
		}
	}
}

// TestLeafRulesHaveNoDepthLimit: a deep chain still yields its innermost leaves.
func TestLeafRulesHaveNoDepthLimit(t *testing.T) {
	chain := "if k == 0 { x }"
	for i := 1; i < 40; i++ {
		chain += " else if k == " + strconv.Itoa(i) + " { x }"
	}
	chain += " else { y }"
	parens := strings.Repeat("(", 40) + "y" + strings.Repeat(")", 40)
	for src, want := range map[string]int{chain: 41, parens: 1} {
		tree := syntax.Parse(token.NewFile("deep.kg", []byte("fn probe() {\n    "+src+"\n}\n")))
		tail := tree.View(tree.List(tree.Root).ItemID(0)).AsFnDecl().Body().AsBlock().Tail()
		leaves := (&checker{t: tree}).tailLeaves(tail.ID())
		if len(leaves) != want || tree.Kind(leaves[len(leaves)-1]) != syntax.Ident {
			t.Errorf("%d leaves for a %d-deep %s, want %d identifiers", len(leaves), 40, tail.Kind(), want)
		}
	}
}

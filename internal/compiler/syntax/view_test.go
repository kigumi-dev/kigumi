package syntax_test

import (
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func parseOK(t *testing.T, src string) *syntax.Tree {
	t.Helper()
	tree := syntax.Parse(token.NewFile("x.kg", []byte(src)))
	if tree.HasErrors() {
		t.Fatalf("parse errors in %q", src)
	}
	return tree
}

func TestViewAccessors(t *testing.T) {
	tree := parseOK(t, "pub fn Point.add(self, dx: Int) -> Int {\n    let y = f(1, name: 2)\n    dx + y\n}\n")
	fn := tree.List(tree.Root).At(0).AsFnDecl()
	if fn.Name().Text() != "add" || !fn.Vis().Valid() || fn.Recv().Path().Join(".") != "Point" {
		t.Fatalf("FnDecl accessors: name %q vis %v recv %q", fn.Name().Text(), fn.Vis().Valid(), fn.Recv().Path().Join("."))
	}
	if fn.Params().Len() != 2 || fn.Params().At(1).AsParam().Name().Text() != "dx" {
		t.Fatalf("params: %d", fn.Params().Len())
	}
	body := fn.Body()
	let := body.At(0).AsLet()
	call := let.Init().AsCallLike()
	if call.Callee().Tok().Text() != "f" || call.Args().Len() != 2 {
		t.Fatalf("call: callee %q args %d", call.Callee().Tok().Text(), call.Args().Len())
	}
	if got := call.Args().At(1).ArgValue().Tok().Text(); got != "2" {
		t.Fatalf("ArgValue of a named argument = %q", got)
	}
	tail := body.Tail().AsBinary()
	if tail.Op().Text() != "+" || tail.Left().Tok().Text() != "dx" || tail.Right().Tok().Text() != "y" {
		t.Fatalf("tail binary: %q %q %q", tail.Left().Tok().Text(), tail.Op().Text(), tail.Right().Tok().Text())
	}
	if (syntax.View{}).Valid() || fn.Ret().Kind() != syntax.TypePath {
		t.Fatalf("absent views must be invalid and present ones typed")
	}
}

func TestListIterationMatchesCursor(t *testing.T) {
	tree := parseOK(t, "fn f() {\n    a()\n    b()\n    c()\n}\n")
	block := tree.List(tree.Root).At(0).AsFnDecl().Body()
	var viaCursor, viaIDs, viaViews []syntax.NodeID
	for c := block.Cursor(); c.Next(); {
		if c.Index() != len(viaCursor) {
			t.Fatalf("cursor index %d at position %d", c.Index(), len(viaCursor))
		}
		viaCursor = append(viaCursor, c.ID())
	}
	for id := range block.IDs() {
		viaIDs = append(viaIDs, id)
	}
	for v := range block.Views() {
		viaViews = append(viaViews, v.ID())
	}
	viaAppend := block.AppendIDs(nil)
	for _, got := range [][]syntax.NodeID{viaIDs, viaViews, viaAppend} {
		if len(got) != 3 || len(viaCursor) != 3 {
			t.Fatalf("lengths %d %d", len(got), len(viaCursor))
		}
		for i := range got {
			if got[i] != viaCursor[i] || got[i] != block.ItemID(i) {
				t.Fatalf("iteration order differs at %d", i)
			}
		}
	}
	if block.First().ID() != viaCursor[0] || block.Last().ID() != viaCursor[2] {
		t.Fatalf("First/Last")
	}
	if tree.List(0).Len() != 0 || tree.List(block.At(0).ID()).Len() != 0 {
		t.Fatalf("the absent node and non-list nodes are empty lists")
	}
}

func TestIterationDoesNotAllocate(t *testing.T) {
	tree := parseOK(t, "fn f() {\n    a()\n    b()\n    c()\n}\n")
	block := tree.List(tree.Root).At(0).AsFnDecl().Body()
	var sum syntax.NodeID
	allocs := testing.AllocsPerRun(100, func() {
		for c := block.Cursor(); c.Next(); {
			sum += c.ID()
		}
		for id := range block.IDs() {
			sum += id
		}
		for v := range block.Views() {
			sum += v.ID()
		}
		for i, v := range block.Enumerate() {
			sum += v.ID() + syntax.NodeID(i)
		}
	})
	if allocs != 0 {
		t.Fatalf("iterating a list allocated %v times", allocs)
	}
	_ = sum
}

func TestSyntheticNodesAndHelpers(t *testing.T) {
	tree := parseOK(t, "import a from std/io/fmt\nlet x = ((1))\n")
	imp := tree.List(tree.Root).At(0).AsImportDecl()
	if got := imp.Path().Join("/"); got != "std/io/fmt" {
		t.Fatalf("import path = %q", got)
	}
	prefix := tree.View(tree.AddPathPrefix(imp.Path().ID())).AsPath()
	if got := prefix.Join("/"); got != "std/io" {
		t.Fatalf("prefix path = %q", got)
	}
	tok := imp.Path().At(0).Index()
	one := tree.View(tree.AddPath(tok, []uint32{tok})).AsPath()
	if one.Len() != 1 || one.Span().Start != tree.Toks[tok].Start {
		t.Fatalf("single-segment path")
	}
	init := tree.List(tree.Root).At(1).AsLet().Init().ID()
	if tree.Kind(tree.Unparen(init)) != syntax.IntLit {
		t.Fatalf("Unparen should reach the literal, got %s", tree.Kind(tree.Unparen(init)))
	}
	tree.DropRootItems(1)
	if tree.List(tree.Root).Len() != 1 || tree.List(tree.Root).At(0).Kind() != syntax.LetStmt {
		t.Fatalf("DropRootItems should leave only the let")
	}
}

func TestChildrenFollowSlotOrder(t *testing.T) {
	tree := parseOK(t, "fn f(a: Int) -> Int {\n    g(a)\n}\n")
	fn := tree.List(tree.Root).At(0)
	var kinds []syntax.NodeKind
	for ch := range fn.Children() {
		kinds = append(kinds, ch.Kind())
	}
	want := []syntax.NodeKind{syntax.List, syntax.TypePath, syntax.Block}
	if len(kinds) != len(want) {
		t.Fatalf("FnDecl children = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("FnDecl children = %v, want %v", kinds, want)
		}
	}
	call := fn.AsFnDecl().Body().At(0).AsExprStmt().Expr()
	n := 0
	for range call.Children() {
		n++
	}
	if n != 2 {
		t.Fatalf("a call has a callee and an argument list, got %d children", n)
	}
}

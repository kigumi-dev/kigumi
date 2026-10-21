package sem

import "kigumi/internal/syntax"

// Slot structs filled from the syntax views, for the callers that want several slots of one node at once.

type fnSlots struct {
	Doc, Name                                     uint32
	Mods                                          uint32
	Attrs, Vis, Recv, Generics, Params, Ret, Body syntax.NodeID
}

func fnDecl(t *syntax.Tree, n syntax.NodeID) fnSlots {
	f := t.View(n).AsFnDecl()
	return fnSlots{Doc: f.Doc().Index(), Attrs: f.Attrs().ID(), Mods: f.Mods(), Vis: f.Vis().ID(), Recv: f.Recv().ID(),
		Name: f.Name().Index(), Generics: f.Generics().ID(), Params: f.Params().ID(), Ret: f.Ret().ID(), Body: f.Body().ID()}
}

type paramSlots struct {
	Attrs, Type syntax.NodeID
	Flags       uint32
}

func param(t *syntax.Tree, n syntax.NodeID) paramSlots {
	p := t.View(n).AsParam()
	return paramSlots{Attrs: p.Attrs().ID(), Flags: p.Flags(), Type: p.Type().ID()}
}

type typeDeclSlots struct {
	Doc, Name                          uint32
	Attrs, Vis, Generics, Layout, Body syntax.NodeID
}

func typeDecl(t *syntax.Tree, n syntax.NodeID) typeDeclSlots {
	d := t.View(n).AsTypeDecl()
	return typeDeclSlots{Doc: d.Doc().Index(), Attrs: d.Attrs().ID(), Vis: d.Vis().ID(), Name: d.Name().Index(),
		Generics: d.Generics().ID(), Layout: d.Layout().ID(), Body: d.Body().ID()}
}

type fieldSlots struct {
	Vis, Type, Metadata syntax.NodeID
	Flags               uint32
}

func field(t *syntax.Tree, n syntax.NodeID) fieldSlots {
	f := t.View(n).AsField()
	return fieldSlots{Vis: f.Vis().ID(), Flags: f.Flags(), Type: f.Type().ID(), Metadata: f.Metadata().ID()}
}

type ifaceSlots struct {
	Doc, Name                     uint32
	Attrs, Vis, Generics, Members syntax.NodeID
}

func ifaceDecl(t *syntax.Tree, n syntax.NodeID) ifaceSlots {
	d := t.View(n).AsInterfaceDecl()
	return ifaceSlots{Doc: d.Doc().Index(), Attrs: d.Attrs().ID(), Vis: d.Vis().ID(), Name: d.Name().Index(),
		Generics: d.Generics().ID(), Members: d.Members().ID()}
}

type constSlots struct {
	Doc, Name               uint32
	Attrs, Vis, Type, Value syntax.NodeID
	Thunk                   syntax.NodeID
}

func constDecl(t *syntax.Tree, n syntax.NodeID) constSlots {
	d := t.View(n).AsConstDecl()
	return constSlots{Doc: d.Doc().Index(), Attrs: d.Attrs().ID(), Vis: d.Vis().ID(), Name: d.Name().Index(),
		Type: d.Type().ID(), Value: d.Value().ID(), Thunk: syntax.NodeID(d.Thunk())}
}

type staticSlots struct {
	Doc, Name               uint32
	Flags                   uint32
	Attrs, Vis, Type, Value syntax.NodeID
	Thunk                   syntax.NodeID
}

func staticDecl(t *syntax.Tree, n syntax.NodeID) staticSlots {
	d := t.View(n).AsStaticDecl()
	return staticSlots{Doc: d.Doc().Index(), Attrs: d.Attrs().ID(), Flags: d.Flags(), Vis: d.Vis().ID(), Name: d.Name().Index(),
		Type: d.Type().ID(), Value: d.Value().ID(), Thunk: syntax.NodeID(d.Thunk())}
}

type contextKeyDeclSlots struct {
	Doc, Name        uint32
	Attrs, Vis, Type syntax.NodeID
}

func contextKeyDecl(t *syntax.Tree, n syntax.NodeID) contextKeyDeclSlots {
	d := t.View(n).AsContextKeyDecl()
	return contextKeyDeclSlots{Doc: d.Doc().Index(), Attrs: d.Attrs().ID(), Vis: d.Vis().ID(), Name: d.Name().Index(),
		Type: d.Type().ID()}
}

type importSlots struct {
	Vis, Binding, Path syntax.NodeID
}

func importDecl(t *syntax.Tree, n syntax.NodeID) importSlots {
	d := t.View(n).AsImportDecl()
	return importSlots{Vis: d.Vis().ID(), Binding: d.Binding().ID(), Path: d.Path().ID()}
}

type letSlots struct {
	Flags                     uint32
	Pattern, Type, Init, Else syntax.NodeID
}

func letStmt(t *syntax.Tree, n syntax.NodeID) letSlots {
	l := t.View(n).AsLet()
	return letSlots{Flags: l.Flags(), Pattern: l.Pattern().ID(), Type: l.Type().ID(), Init: l.Init().ID(), Else: l.Else().ID()}
}

type ifSlots struct {
	Cond, Then, Else syntax.NodeID
}

func ifExpr(t *syntax.Tree, n syntax.NodeID) ifSlots {
	i := t.View(n).AsIf()
	return ifSlots{Cond: i.Cond().ID(), Then: i.Then().ID(), Else: i.Else().ID()}
}

type ifLetSlots struct {
	Flags                            uint32
	Pattern, Init, Guard, Then, Else syntax.NodeID
}

func ifLet(t *syntax.Tree, n syntax.NodeID) ifLetSlots {
	i := t.View(n).AsIfLet()
	return ifLetSlots{Flags: i.Flags(), Pattern: i.Pattern().ID(), Init: i.Init().ID(), Guard: i.Guard().ID(),
		Then: i.Then().ID(), Else: i.Else().ID()}
}

type armSlots struct {
	Pattern, Guard, Body syntax.NodeID
}

func matchArm(t *syntax.Tree, n syntax.NodeID) armSlots {
	a := t.View(n).AsArm()
	return armSlots{Pattern: a.Pattern().ID(), Guard: a.Guard().ID(), Body: a.Body().ID()}
}

type forSlots struct {
	Pattern, Head, Body syntax.NodeID
}

func forExpr(t *syntax.Tree, n syntax.NodeID) forSlots {
	f := t.View(n).AsFor()
	return forSlots{Pattern: f.Pattern().ID(), Head: f.Head().ID(), Body: f.Body().ID()}
}

type fnTypeSlots struct {
	Mods, Abi, Flags uint32
	Params, Ret      syntax.NodeID
}

func fnType(t *syntax.Tree, n syntax.NodeID) fnTypeSlots {
	f := t.View(n).AsTypeFn()
	return fnTypeSlots{Mods: f.Mods(), Abi: f.Abi().Index(), Params: f.Params().ID(), Flags: f.Flags(), Ret: f.Ret().ID()}
}

func pathText(t *syntax.Tree, n syntax.NodeID, sep string) string {
	return t.View(n).AsPath().Join(sep)
}

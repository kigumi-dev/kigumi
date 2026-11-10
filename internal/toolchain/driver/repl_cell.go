package driver

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// replDeclKinds are the top-level declarations a cell may append to the
// session source. Statements and lets stay in the cell wrapper.
func replDeclKind(k syntax.NodeKind) bool {
	switch k {
	case syntax.FnDecl, syntax.TypeDecl, syntax.InterfaceDecl, syntax.ConstDecl,
		syntax.ImportDecl, syntax.StaticDecl:
		return true
	}
	return false
}

type replCellLet struct {
	name string
	mut  bool
}

type replCell struct {
	file  *token.File
	tree  *syntax.Tree
	decls string
	// body holds the statement source in order; last is not in body when it
	// is the displayed expression.
	body     []string
	last     string
	lastNode syntax.NodeID
	lastExpr bool
	lets     []replCellLet
}

func parseReplCell(src string) (*replCell, string) {
	file := token.NewFile("cell.kg", []byte(src))
	tree := syntax.Parse(file)
	if tree.HasErrors() {
		return nil, diag.RenderAll(file, tree.Diags)
	}
	c := &replCell{file: file, tree: tree}
	var decls strings.Builder
	var statements []string
	var items []syntax.View
	for item := range tree.List(tree.Root).Views() {
		items = append(items, item)
	}
	for i, item := range items {
		start := int(item.Span().Start)
		end := len(file.Src)
		if i+1 < len(items) {
			end = int(items[i+1].Span().Start)
		}
		text := strings.TrimSpace(string(file.Src[start:end]))
		switch {
		case replDeclKind(item.Kind()):
			decls.WriteString(text)
			decls.WriteByte('\n')
		case item.Kind() == syntax.LetStmt:
			lets := item.AsLet()
			names := replPatternNames(lets.Pattern())
			mut := lets.Flags()&syntax.FlagMut != 0
			for _, n := range names {
				c.lets = append(c.lets, replCellLet{name: n, mut: mut})
			}
			statements = append(statements, text)
		case item.Kind() == syntax.ExprStmt:
			// A top-level control transfer is legal in a function but not
			// as a cell's own command; reject it before wrapping.
			if e := item.AsExprStmt().Expr(); replControlKind(e.Kind()) {
				return nil, fmt.Sprintf("cannot use `%s` at the top level of a REPL cell", e.Kind())
			}
			c.last = text
			c.lastNode = item.ID()
			c.lastExpr = true
		default:
			statements = append(statements, text)
		}
	}
	c.decls = strings.TrimRight(decls.String(), "\n")
	c.body = statements
	return c, ""
}

func replControlKind(k syntax.NodeKind) bool {
	return k == syntax.ReturnExpr || k == syntax.BreakExpr || k == syntax.ContinueExpr
}

func replPatternNames(p syntax.PatternView) []string {
	var out []string
	var walk func(syntax.View)
	walk = func(v syntax.View) {
		if !v.Valid() {
			return
		}
		if v.Kind() == syntax.PatBind {
			out = append(out, v.AsPatBind().Name().Text())
			return
		}
		for child := range v.Children() {
			walk(child)
		}
	}
	walk(p.View)
	return out
}

// replWrapper builds one cell wrapper. out is empty for the first pass; with
// out set, the wrapper returns the output record and every field in order.
func replWrapper(decls, cellName string, bindings []ReplBinding, outName string, out []replOutField, body []string, last string, lastResult bool, lastVar string) string {
	var b strings.Builder
	if decls != "" {
		b.WriteString(decls)
		b.WriteByte('\n')
	}
	if outName != "" {
		fmt.Fprintf(&b, "type %s = {\n", outName)
		for _, f := range out {
			fmt.Fprintf(&b, "    %s %s\n", f.name, f.typ)
		}
		b.WriteString("}\n")
	}
	fmt.Fprintf(&b, "fn %s(", cellName)
	for i, x := range bindings {
		if i > 0 {
			b.WriteString(", ")
		}
		if x.Mut {
			b.WriteString("mut ")
		}
		fmt.Fprintf(&b, "%s: %s", x.Name, x.Type)
	}
	if outName == "" {
		b.WriteString(") -> Void! {\n")
	} else {
		fmt.Fprintf(&b, ") -> %s! {\n", outName)
	}
	for _, line := range body {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if lastVar == "!" && last != "" {
		b.WriteString(last)
		b.WriteByte('\n')
	} else if lastVar != "" && last != "" {
		if lastResult {
			fmt.Fprintf(&b, "    let %s = (%s)?\n", lastVar, last)
		} else {
			fmt.Fprintf(&b, "    let %s = %s\n", lastVar, last)
		}
	}
	if outName != "" && len(out) > 0 {
		fmt.Fprintf(&b, "    %s {\n", outName)
		for _, f := range out {
			fmt.Fprintf(&b, "        %s: %s\n", f.name, f.name)
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n")
	return b.String()
}

type replOutField struct {
	name string
	typ  string
}

// generatedFile finds the parsed overlay file and its package.
func generatedFile(m *Module, name string) (*Package, *syntax.Tree) {
	for _, path := range m.Order {
		p := m.Packages[path]
		for _, t := range p.Files {
			if t.File.Name == name {
				return p, t
			}
		}
	}
	return nil, nil
}

// generatedCellLocals finds the direct lets of the generated wrapper and maps
// each binding name to its local entity.
func generatedCellLocals(m *Module, res *sem.Result, file *syntax.Tree, cellName string) (map[string]sem.EntityID, map[string]bool, syntax.NodeID) {
	out := map[string]sem.EntityID{}
	mut := map[string]bool{}
	var body syntax.NodeID
	for d := range file.List(file.Root).Views() {
		if d.Kind() != syntax.FnDecl || d.AsFnDecl().Name().Text() != cellName {
			continue
		}
		body = d.AsFnDecl().Body().ID()
		break
	}
	if body == 0 {
		return out, mut, 0
	}
	for st := range file.List(body).Views() {
		if st.Kind() != syntax.LetStmt {
			continue
		}
		l := st.AsLet()
		names := replPatternNames(l.Pattern())
		for _, n := range names {
			ent := replDefOfPattern(res, file, l.Pattern(), n)
			if ent != 0 {
				out[n] = ent
				mut[n] = l.Flags()&syntax.FlagMut != 0
			}
		}
	}
	return out, mut, body
}

func replDefOfPattern(res *sem.Result, file *syntax.Tree, p syntax.PatternView, name string) sem.EntityID {
	var found sem.EntityID
	var walk func(syntax.View)
	walk = func(v syntax.View) {
		if !v.Valid() || found != 0 {
			return
		}
		if v.Kind() == syntax.PatBind && v.AsPatBind().Name().Text() == name {
			if fi := res.File(file); fi != nil && int(v.ID()) < len(fi.Defs) {
				found = fi.Defs[v.ID()]
			}
			return
		}
		for child := range v.Children() {
			walk(child)
		}
	}
	walk(p.View)
	return found
}

// replDiagOnGenerated hides the wrapper's own source from a user diagnostic:
// a span in the copied body is mapped back to the cell; wrapper-only errors
// get a short message without an internal snippet.

func replOutputErrors(res *sem.Result, tree *syntax.Tree, outName string, specs []replOutSpec) map[int]bool {
	if outName == "" || tree == nil {
		return nil
	}
	byName := map[string]int{}
	for i, s := range specs {
		byName[s.name] = i
	}
	lineField := map[int]int{}
	inBlock := false
	lines := strings.Split(string(tree.File.Src), "\n")
	for n, line := range lines {
		if !inBlock {
			if strings.HasPrefix(strings.TrimSpace(line), outName+" {") {
				inBlock = true
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "}" {
			break
		}
		name, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if i, ok := byName[name]; ok && specs[i].kind != replOutLast {
			lineField[n+1] = i
		}
	}
	if len(lineField) == 0 {
		return nil
	}
	out := map[int]bool{}
	for _, d := range res.Diagnostics(tree) {
		line := tree.File.Line(d.Loc.Span.Start)
		if i, ok := lineField[line]; ok {
			out[i] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

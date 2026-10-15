package syntax

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/token"
)

func schemaOf(t *testing.T) map[NodeKind]kindSchema {
	t.Helper()
	by := map[NodeKind]kindSchema{}
	for _, ks := range schema {
		if _, dup := by[ks.kind]; dup {
			t.Errorf("kind %s is declared twice", ks.kind)
		}
		by[ks.kind] = ks
	}
	for k := NodeKind(1); k < kindEnd; k++ {
		if _, ok := by[k]; !ok {
			t.Errorf("kind %d (%s) has no schema entry", k, k)
		}
	}
	return by
}

func TestSchemaTable(t *testing.T) {
	by := schemaOf(t)
	viewNames := map[string]NodeKind{}
	reserved := map[string]bool{"ID": true, "Tree": true, "Kind": true, "Tok": true, "Span": true, "Valid": true, "Is": true}
	listReserved := map[string]bool{"Len": true, "At": true, "ItemID": true, "First": true, "Last": true, "Cursor": true, "IDs": true, "Views": true, "Enumerate": true, "EnumerateIDs": true, "AppendIDs": true}
	for k := NodeKind(1); k < kindEnd; k++ {
		ks, ok := by[k]
		if !ok {
			continue
		}
		if prev, dup := viewNames[ks.view]; dup {
			t.Errorf("view name %q is used by %s and %s", ks.view, prev, k)
		}
		viewNames[ks.view] = k
		seen := map[string]bool{}
		names := []string{}
		if ks.tok != "" {
			names = append(names, ks.tok)
		}
		for _, s := range ks.slots {
			names = append(names, s.name)
		}
		for _, n := range names {
			if seen[n] {
				t.Errorf("%s: accessor name %q is declared twice", k, n)
			}
			seen[n] = true
			if n != "Items" && n != "Toks" && (reserved[n] || ks.layout == layList && listReserved[n]) {
				t.Errorf("%s: accessor name %q shadows a View method", k, n)
			}
		}
		checkLayout(t, ks)
	}
}

func checkLayout(t *testing.T, ks kindSchema) {
	t.Helper()
	switch ks.layout {
	case layLeaf:
		if len(ks.slots) != 0 {
			t.Errorf("%s: a leaf has no slots", ks.kind)
		}
	case layL:
		if len(ks.slots) != 1 || !isNodeRole(ks.slots[0].role) {
			t.Errorf("%s: layL needs one node slot", ks.kind)
		}
	case layList:
		if len(ks.slots) != 1 || ks.slots[0].role != roleItems {
			t.Errorf("%s: layList needs the items slot", ks.kind)
		}
	case layPath:
		if len(ks.slots) != 1 || ks.slots[0].role != roleToks {
			t.Errorf("%s: layPath needs the toks slot", ks.kind)
		}
	case layLR:
		if len(ks.slots) != 2 || !isNodeRole(ks.slots[0].role) || !isNodeRole(ks.slots[1].role) {
			t.Errorf("%s: layLR needs two node slots", ks.kind)
		}
	case layFlagR:
		if len(ks.slots) != 2 || ks.slots[0].role != roleInt || !isNodeRole(ks.slots[1].role) {
			t.Errorf("%s: layFlagR needs an int slot and a node slot", ks.kind)
		}
	case layTokR:
		if len(ks.slots) != 2 || ks.slots[0].role != roleTok || !isNodeRole(ks.slots[1].role) {
			t.Errorf("%s: layTokR needs a token slot and a node slot", ks.kind)
		}
	case laySpan:
		if len(ks.slots) != 2 || ks.slots[0].role != roleInt || ks.slots[1].role != roleInt {
			t.Errorf("%s: laySpan needs two int slots", ks.kind)
		}
	case layRec:
		if len(ks.slots) == 0 {
			t.Errorf("%s: layRec needs slots", ks.kind)
		}
		for _, s := range ks.slots {
			if s.role == roleItems || s.role == roleToks {
				t.Errorf("%s: slot %s has a list-layout role", ks.kind, s.name)
			}
		}
	}
	for _, s := range ks.slots {
		if (s.role == roleChild || s.role == roleList || s.role == roleItems) && s.kinds == (kindSet{}) {
			t.Errorf("%s.%s: no kinds declared", ks.kind, s.name)
		}
	}
}

func isNodeRole(r role) bool { return r == roleChild || r == roleList }

// syntheticSources cover kinds the repository sources never build.
var syntheticSources = []string{
	"let x = $\"ls ${dir} > out.txt\"\n",
}

func corpusSources() map[string][]byte {
	out := map[string][]byte{}
	for _, root := range []string{"../../std", "../../examples", "../../testdata"} {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return nil
			}
			switch {
			case strings.HasSuffix(p, ".kg"):
				out[p] = b
			case strings.HasSuffix(p, ".txtar"):
				for name, body := range txtarSources(string(b)) {
					out[p+"#"+name] = []byte(body)
				}
			}
			return nil
		})
	}
	for i, src := range syntheticSources {
		out[fmt.Sprintf("synthetic#%d", i)] = []byte(src)
	}
	return out
}

func txtarSources(text string) map[string]string {
	out := map[string]string{}
	name, cur := "", []string{}
	flush := func() {
		if name == "src" || strings.HasSuffix(name, ".kg") {
			out[name] = strings.Join(cur, "\n") + "\n"
		}
	}
	for _, ln := range strings.Split(text, "\n") {
		if strings.HasPrefix(ln, "-- ") && strings.HasSuffix(ln, " --") {
			flush()
			name, cur = strings.TrimSuffix(strings.TrimPrefix(ln, "-- "), " --"), nil
			continue
		}
		cur = append(cur, ln)
	}
	flush()
	return out
}

// TestSchemaMatchesParser checks every parsed node against the table: slots
// in range, required ones filled, only declared kinds, nothing undeclared.
func TestSchemaMatchesParser(t *testing.T) {
	by := schemaOf(t)
	seen := map[NodeKind]bool{}
	problems := map[string]int{}
	report := func(format string, a ...any) {
		problems[fmt.Sprintf(format, a...)]++
	}
	files := 0
	for name, src := range corpusSources() {
		tr := Parse(token.NewFile(name, src))
		if tr.HasErrors() {
			continue
		}
		files++
		for id := 1; id < len(tr.nodes); id++ {
			n := tr.nodes[id]
			seen[n.Kind] = true
			checkNode(tr, NodeID(id), by[n.Kind], report)
		}
	}
	var msgs []string
	for m, c := range problems {
		msgs = append(msgs, fmt.Sprintf("%s (x%d)", m, c))
	}
	sort.Strings(msgs)
	for _, m := range msgs {
		t.Error(m)
	}
	for k := NodeKind(1); k < kindEnd; k++ {
		// The parser builds a ContractBlock only next to a diagnostic.
		if !seen[k] && k != ContractBlock {
			t.Errorf("no corpus source builds a %s node; add one to syntheticSources", k)
		}
	}
	if files < 500 {
		t.Errorf("corpus too small: %d error-free files", files)
	}
}

func checkNode(tr *Tree, id NodeID, ks kindSchema, report func(string, ...any)) {
	n := tr.nodes[id]
	if int(n.Tok) >= len(tr.Toks) {
		report("%s: Tok out of range", n.Kind)
	}
	val := func(i int) uint32 {
		switch ks.layout {
		case layL, layLR, layFlagR, layTokR, laySpan:
			if i == 0 {
				return n.Lhs
			}
			return n.Rhs
		}
		return tr.extra[int(n.Lhs)+i]
	}
	switch ks.layout {
	case layLeaf:
		if n.Lhs != 0 || n.Rhs != 0 {
			report("%s: leaf with Lhs/Rhs set", n.Kind)
		}
	case layL:
		if n.Rhs != 0 {
			report("%s: Rhs set on a one-child kind", n.Kind)
		}
	case layRec:
		if int(n.Rhs-n.Lhs) != len(ks.slots) {
			report("%s: %d record slots written, %d declared", n.Kind, n.Rhs-n.Lhs, len(ks.slots))
			return
		}
	case layList:
		for _, e := range tr.extra[n.Lhs:n.Rhs] {
			checkChild(tr, n.Kind, "Items", ks.slots[0], e, report)
		}
		return
	case layPath:
		for _, e := range tr.extra[n.Lhs:n.Rhs] {
			if int(e) >= len(tr.Toks) || e == 0 {
				report("%s: bad path token", n.Kind)
			}
		}
		return
	}
	for i, s := range ks.slots {
		v := val(i)
		switch s.role {
		case roleChild:
			checkChild(tr, n.Kind, s.name, s, v, report)
		case roleList:
			if v == 0 {
				if !s.opt {
					report("%s.%s: required list is absent", n.Kind, s.name)
				}
				continue
			}
			if tr.nodes[v].Kind != List {
				report("%s.%s: list slot holds %s", n.Kind, s.name, tr.nodes[v].Kind)
				continue
			}
			for _, e := range tr.extra[tr.nodes[v].Lhs:tr.nodes[v].Rhs] {
				checkChild(tr, n.Kind, s.name+"[]", s, e, report)
			}
		case roleTok:
			if v == 0 && !s.opt {
				report("%s.%s: required token is absent", n.Kind, s.name)
			}
			if int(v) >= len(tr.Toks) {
				report("%s.%s: token out of range", n.Kind, s.name)
			}
		}
	}
}

func checkChild(tr *Tree, parent NodeKind, name string, s slot, c uint32, report func(string, ...any)) {
	if c == 0 {
		if !s.opt && !strings.HasSuffix(name, "[]") && name != "Items" {
			report("%s.%s: required child is absent", parent, name)
		}
		return
	}
	if int(c) >= len(tr.nodes) {
		report("%s.%s: child out of range", parent, name)
		return
	}
	if k := tr.nodes[c].Kind; !s.kinds.has(k) {
		report("%s.%s: undeclared child kind %s", parent, name, k)
	}
}

func TestCallLikeKindsShareALayout(t *testing.T) {
	by := schemaOf(t)
	call, ws := by[CallExpr], by[WsCallExpr]
	if call.layout != ws.layout || len(call.slots) != len(ws.slots) {
		t.Fatalf("CallExpr and WsCallExpr differ in layout")
	}
	for i := range call.slots {
		if call.slots[i].name != ws.slots[i].name || call.slots[i].role != ws.slots[i].role {
			t.Fatalf("slot %d differs: %s vs %s", i, call.slots[i].name, ws.slots[i].name)
		}
	}
}

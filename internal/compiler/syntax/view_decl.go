package syntax

import "strings"

// The slots the declaration kinds share, looked up by name once at load time.
const (
	declDoc = iota
	declAttrs
	declVis
	declName
	declGenerics
	declSlots
)

var declSlotNames = [declSlots]string{"Doc", "Attrs", "Vis", "Name", "Generics"}

var declSlot [kindEnd][declSlots]int8

func init() {
	for k := range declSlot {
		for j := range declSlot[k] {
			declSlot[k][j] = -1
		}
	}
	for i := range schema {
		ks := &schema[i]
		if ks.layout != layRec {
			continue
		}
		for pos, s := range ks.slots {
			for j, name := range declSlotNames {
				if s.name == name {
					declSlot[ks.kind][j] = int8(pos)
				}
			}
		}
	}
}

func (d DeclView) shared(which int) (uint32, bool) {
	pos := declSlot[d.Kind()][which]
	if pos < 0 {
		return 0, false
	}
	return d.rec(int(pos)), true
}

// Doc is the last `///` token of the doc comment, absent when undocumented.
func (d DeclView) Doc() TokenRef {
	v, _ := d.shared(declDoc)
	return d.tokRef(v)
}

func (d DeclView) Attrs() ListView {
	v, _ := d.shared(declAttrs)
	return ListView{d.at(v)}
}

func (d DeclView) Vis() VisibilityView {
	v, _ := d.shared(declVis)
	return d.at(v).AsVisibility()
}

func (d DeclView) Name() TokenRef {
	v, _ := d.shared(declName)
	return d.tokRef(v)
}

func (d DeclView) Generics() ListView {
	v, _ := d.shared(declGenerics)
	return ListView{d.at(v)}
}

// Join renders the path's segments separated by sep.
func (p PathView) Join(sep string) string {
	if p.Len() == 1 {
		return p.At(0).Text()
	}
	var sb strings.Builder
	for i, seg := range p.Segments() {
		if i > 0 {
			sb.WriteString(sep)
		}
		sb.WriteString(seg.Text())
	}
	return sb.String()
}

// AppendIDs appends the item ids to dst, for callers that sort or index them.
func (l ListView) AppendIDs(dst []NodeID) []NodeID {
	for c := l.Cursor(); c.Next(); {
		dst = append(dst, c.cur)
	}
	return dst
}

// AsCallLike views a CallExpr or WsCallExpr as a CallView: both keep the
// callee and the argument list in the same positions.
func (v View) AsCallLike() CallView { return CallView{v} }

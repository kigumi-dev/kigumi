package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

func (c *checker) variantTypeArgs(n syntax.NodeID, v EntityID, items syntax.ListView) TypeID {
	parent := c.r.Entities[c.r.Entities[v].Parent].Name
	args := make([]string, 0, items.Len())
	for _, id := range items.AppendIDs(nil) {
		args = append(args, c.srcText(id))
	}
	if len(args) > 0 {
		parent += "[" + strings.Join(args, ", ") + "]"
	}
	name := c.r.Entities[v].Name
	c.errAt(n, cVariantTypeArgs, name, parent, name, name)
	return TyPoison
}

func (c *checker) srcText(n syntax.NodeID) string {
	sp := c.t.Span(n)
	return string(c.t.File.Src[sp.Start:sp.End])
}

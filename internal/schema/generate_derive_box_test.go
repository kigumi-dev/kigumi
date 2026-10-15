package schema_test

import (
	"strings"
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
)

const boxFieldSrc = `
import json from std/json
import {Box} from std/alloc

type Holder = {
    pub n Box[Int]
    pub label Box[String]
}

type Tree = Leaf(v: Int) | Node(kid: Box[Tree])

let h = Holder { n: Box.new(1), label: Box.new("a") }
print json.encode(h)?
print json.encode(Tree.Node(Box.new(Tree.Leaf(2))))?
`

func TestGenerateDeriveJSONThroughBox(t *testing.T) {
	m, res := loadModule(t, boxFieldSrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	var holder, tree sem.EntityID
	for ent, d := range byEntity {
		switch d.Name {
		case "Holder":
			holder = ent
		case "Tree":
			tree = ent
		}
	}
	if holder == 0 || tree == 0 {
		t.Fatal("Holder or Tree not found in Schema IR")
	}
	if ref := byEntity[holder].Fields[0].Type; ref.Kind != schema.TRNamed || ref.Display != "Box[Int]" || ref.Decl == nil || ref.Decl.Name != "Box" {
		t.Errorf("Box[Int] field is %+v, want a named Box[Int]", ref)
	}
	cases := []struct {
		decl  sem.EntityID
		iface string
		want  []string
	}{
		{holder, "Encode", []string{"f.put(\"n\", self.n.toJson())", "f.put(\"label\", self.label.toJson())"}},
		{holder, "Decode", []string{"n: o.field(\"n\")?", "label: o.field(\"label\")?"}},
		{tree, "Encode", []string{"Tree.Node(p0) -> json.tagged(\"Node\", Array.of(p0.toJson()))"}},
		{tree, "Decode", []string{"return Ok(Tree.Node(t.next()?))"}},
	}
	for _, c := range cases {
		text, err := schema.GenerateDerive(res, sem.DeriveRequest{Decl: c.decl, Interface: c.iface})
		if err != nil {
			t.Fatalf("%s: %v", c.iface, err)
		}
		for _, w := range c.want {
			if !strings.Contains(text, w) {
				t.Errorf("%s output lacks %q:\n%s", c.iface, w, text)
			}
		}
	}
}

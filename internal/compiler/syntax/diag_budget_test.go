package syntax_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func TestParseDiagsDroppedCountsOmissions(t *testing.T) {
	var sb strings.Builder
	for range 150 {
		sb.WriteString("`\n")
	}
	f := token.NewFile("a.kg", []byte(sb.String()))
	tree := syntax.Parse(f)
	if len(tree.Diags) != 100 {
		t.Fatalf("Diags = %d, want the 100-diagnostic limit", len(tree.Diags))
	}
	if tree.DiagsDropped == 0 {
		t.Fatal("DiagsDropped must be nonzero once the bag's limit is exceeded")
	}
}

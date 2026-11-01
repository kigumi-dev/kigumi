package driver

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// scanImports reads every `import` keyword with the lexer alone, so it may name more packages than the parser, never fewer.
func scanImports(src []byte) []string {
	toks := syntax.Lex(src, &diag.Bag{})
	var out []string
	for i, tk := range toks {
		if tk.Kind != token.KwImport {
			continue
		}
		depth := 0
		for j := i + 1; j < len(toks) && toks[j].Kind != token.EOF && (depth > 0 || toks[j].Kind != token.Newline); j++ {
			switch toks[j].Kind {
			case token.LBrace:
				depth++
			case token.RBrace:
				depth--
			}
			if depth > 0 || toks[j].Kind != token.Ident || toks[j].Text(src) != "from" {
				continue
			}
			from := int(toks[j].End)
			for from < len(src) && (src[from] == ' ' || src[from] == '\t') {
				from++
			}
			end := from
			for end < len(src) && !isPathEnd(src[end]) {
				end++
			}
			if end > from {
				out = append(out, string(src[from:end]))
			}
			break
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func isPathEnd(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func contentHash(src []byte) string {
	sum := sha256.Sum256(src)
	return hex.EncodeToString(sum[:])
}

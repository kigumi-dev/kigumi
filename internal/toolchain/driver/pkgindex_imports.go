package driver

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"slices"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// scanSource lexes src once for the two things the index needs from it: the
// content hash and the imported paths.
func scanSource(name string, src []byte) (string, []string) {
	toks := syntax.Lex(src, &diag.Bag{})
	return semanticHash(name, src, toks), scanImports(src, toks)
}

// scanImports reads every `import` keyword with the lexer alone, so it may name more packages than the parser, never fewer.
func scanImports(src []byte, toks []token.Token) []string {
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

// semanticHash hashes what a file means rather than how it is laid out: the
// code tokens with their position and the text of every line that holds one,
// and no comment-only line. A comment or blank line added after the last token
// keeps every cache key the file takes part in, as it should: it cannot change
// a diagnostic, and a line shift would, so the position stays in the hash.
func semanticHash(name string, src []byte, toks []token.Token) string {
	file := token.NewFile(name, src)
	h := sha256.New()
	var num [binary.MaxVarintLen64]byte
	lastLine := -1
	for _, tk := range toks {
		if tk.Kind == token.BOF || tk.Kind == token.EOF || tk.Kind == token.Newline || tk.Kind == token.Comment || tk.Kind == token.BlockComment {
			continue
		}
		line := file.Line(tk.Span.Start)
		if line != lastLine {
			lastLine = line
			span := file.LineSpan(line)
			put := binary.PutUvarint(num[:], uint64(span.Len()))
			h.Write(num[:put])
			h.Write(src[span.Start:span.End])
		}
		h.Write([]byte{byte(tk.Kind)})
		text := src[tk.Span.Start:tk.Span.End]
		put := binary.PutUvarint(num[:], uint64(len(text)))
		h.Write(num[:put])
		h.Write(text)
		put = binary.PutUvarint(num[:], uint64(tk.Span.Start))
		h.Write(num[:put])
	}
	return hex.EncodeToString(h.Sum(nil))
}

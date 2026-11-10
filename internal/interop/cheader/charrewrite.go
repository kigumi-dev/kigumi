package cheader

import (
	"os"
	"path/filepath"
)

// charCorrectedHeader returns a path to translate-c that spells every plain
// `char` in the header as `signed char`/`unsigned char`, so a target with a
// signed plain char keeps it. zig translate-c is target-blind here: it always
// renders a plain `char` as `u8`, and its output no longer says which `u8`s
// came from a plain char (`signed char`/`unsigned char` are `i8`/`u8` too),
// so the signedness has to be recovered from the C source itself.
//
// The corrected copy lives in a temp directory next to symlinks of the
// header's siblings, so its own `#include "..."` lines keep resolving; the
// header's bytes keep their line structure (only `char` tokens grow), so
// diagnostics and `__LINE__` still point at the right places. Only the
// header the module declared is rewritten: a sibling it includes would need
// the whole include tree mirrored to stay consistent, and a system header
// must not be touched at all. cleanup removes the temp directory (nil when
// the header is used as written).
func charCorrectedHeader(header string, abi ABI) (path string, cleanup func(), err error) {
	if !abi.plainCharSigned() {
		// On a target whose plain char is unsigned (arm, arm64) zig's `u8`
		// already means the same thing, so the header needs no rewrite.
		return header, nil, nil
	}
	src, err := os.ReadFile(header)
	if err != nil {
		return "", nil, err
	}
	corrected := rewritePlainChar(src, abi.CharUnsigned)
	if string(corrected) == string(src) {
		return header, nil, nil
	}
	dir, err := os.MkdirTemp("", "kigumi-charhdr")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(dir) }
	base := filepath.Base(header)
	linkSiblings(filepath.Dir(header), dir, base)
	dst := filepath.Join(dir, base)
	if err := os.WriteFile(dst, corrected, 0o644); err != nil {
		cleanup()
		return "", nil, err
	}
	return dst, cleanup, nil
}

// linkSiblings symlinks every entry of dir into tmp, except the header
// itself (which the caller writes there as the corrected copy). A failed
// link only narrows which sibling includes would resolve, so it is skipped
// rather than failing the import.
func linkSiblings(dir, tmp, skip string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if e.Name() == skip {
			continue
		}
		_ = os.Symlink(filepath.Join(dir, e.Name()), filepath.Join(tmp, e.Name()))
	}
}

// rewritePlainChar replaces each C `char` token that is not already
// qualified by `signed` or `unsigned` with `signed char` (or `unsigned
// char` on an unsigned-char target). Comments, string and character
// literals, and preprocessor directive lines are copied untouched: a
// directive's text is not a type use, and rewriting inside one could break
// an #include path or a macro's own tokens.
func rewritePlainChar(src []byte, unsigned bool) []byte {
	out := make([]byte, 0, len(src)+16)
	replacement := "signed char"
	if unsigned {
		replacement = "unsigned char"
	}
	lastIdent := ""
	atLineStart := true
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == '\n':
			out = append(out, c)
			lastIdent = ""
			atLineStart = true
			i++
			continue
		case c == ' ' || c == '\t' || c == '\r':
			out = append(out, c)
			i++
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			end := i
			for end < len(src) && src[end] != '\n' {
				end++
			}
			out = append(out, src[i:end]...)
			i = end
			continue
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := i + 2
			for end+1 < len(src) && !(src[end] == '*' && src[end+1] == '/') {
				end++
			}
			end = min(end+2, len(src))
			out = append(out, src[i:end]...)
			i = end
			continue
		case c == '"' || c == '\'':
			end := skipLiteral(string(src), i)
			out = append(out, src[i:end]...)
			i = end
			lastIdent = ""
			atLineStart = false
			continue
		case c == '#' && atLineStart:
			end := i
			for end < len(src) {
				if src[end] == '\\' && end+1 < len(src) && src[end+1] == '\n' {
					end += 2
					continue
				}
				if src[end] == '\n' {
					break
				}
				end++
			}
			out = append(out, src[i:end]...)
			i = end
			continue
		}
		if isIdentByte(c) && (i == 0 || !isIdentByte(src[i-1])) {
			end := i
			for end < len(src) && isIdentByte(src[end]) {
				end++
			}
			tok := string(src[i:end])
			// A pointer's pointee signedness cannot change the C ABI, and
			// Kigumi spells text bytes `u8`, so `char *` keeps zig's u8
			// while a by-value char (parameter, result, field, typedef)
			// becomes the target's plain char.
			if tok == "char" && lastIdent != "signed" && lastIdent != "unsigned" && nextSignificant(src, end) != '*' {
				out = append(out, replacement...)
			} else {
				out = append(out, tok...)
			}
			lastIdent = tok
			atLineStart = false
			i = end
			continue
		}
		out = append(out, c)
		atLineStart = false
		i++
	}
	return out
}

// nextSignificant returns the first byte after i that is not whitespace or
// part of a comment, so `char *` and `char /*c*/ *p` are both seen as a
// pointer's pointee.
func nextSignificant(src []byte, i int) byte {
	for i < len(src) {
		switch {
		case src[i] == ' ' || src[i] == '\t' || src[i] == '\r' || src[i] == '\n':
			i++
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
		default:
			return src[i]
		}
	}
	return 0
}

func isIdentByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

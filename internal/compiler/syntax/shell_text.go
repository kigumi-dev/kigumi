package syntax

import (
	"strings"
	"unicode/utf8"
)

// ShellQuote is the quoting a ShellText piece was written in, which decides what a shell backslash in it escapes.
type ShellQuote uint32

const (
	ShellBare ShellQuote = iota
	ShellSingle
	ShellDouble
)

var shellQuoteNames = [...]string{ShellBare: "", ShellSingle: " single", ShellDouble: " double"}

// shellCharAt returns the character at pos after language decoding, its source length (0 past the end), and whether
// it was written as an escape; only a raw `$` can start an interpolation.
func shellCharAt(src []byte, pos, end int) (rune, int, bool) {
	if pos >= end {
		return 0, 0, false
	}
	if src[pos] != '\\' || pos+1 >= end {
		return rune(src[pos]), 1, false
	}
	n := 2
	if j := pos + 2; src[pos+1] == 'u' && j < end && src[j] == '{' {
		for j++; j < end && isHexDigit(src[j]); j++ {
		}
		if j < end && src[j] == '}' {
			n = j + 1 - pos
		}
	}
	d := DecodeString(string(src[pos : pos+n]))
	if c, size := utf8.DecodeRuneInString(d); size == len(d) {
		return c, n, true
	}
	return utf8.RuneError, n, true
}

func (s *shellParser) at(pos int) (rune, int, bool) { return shellCharAt(s.src, pos, s.end) }

func isShellBreak(c rune) bool {
	return strings.ContainsRune(" \t\n|<>&;()`", c) && c != 0
}

func isDigitRune(c rune) bool { return '0' <= c && c <= '9' }

// DecodeShellText decodes one ShellText piece: language escapes first, then the shell backslash, which escapes any
// character outside quotes and only a backslash, quote, dollar, backtick or newline inside double quotes.
func DecodeShellText(raw []byte, q ShellQuote) string {
	s := DecodeString(string(raw))
	if q == ShellSingle {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		if n := s[i+1]; q == ShellBare || strings.IndexByte("\\\"$`\n", n) >= 0 {
			b.WriteByte(n)
			i++
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

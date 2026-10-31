package diag

import (
	"unicode"

	"golang.org/x/text/width"
)

// displayWidth counts terminal cells, not bytes, for caret alignment.
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r == '\t':
			w += 4 - w%4
		case runeWidth(r) == 0:
		case runeWidth(r) == 2:
			w += 2
		default:
			w++
		}
	}
	return w
}

// runeWidth returns the rune's terminal cell width: 0, 1, or 2.
func runeWidth(r rune) int {
	switch {
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	case r == 0x200D: // zero width joiner
		return 0
	case r >= 0xFE00 && r <= 0xFE0F: // variation selectors
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	}
	if isEmojiPresentation(r) {
		return 2
	}
	return 1
}

// isEmojiPresentation covers emoji blocks x/text/width's East Asian table doesn't mark Wide, since most terminals still render them at two cells.
func isEmojiPresentation(r rune) bool {
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x1F1E6 && r <= 0x1F1FF: // regional indicators
		return true
	case r >= 0x1F000 && r <= 0x1F2FF:
		return true
	}
	return false
}

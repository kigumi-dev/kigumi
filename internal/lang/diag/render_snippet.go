package diag

import "unicode/utf8"

func snippetLine(text string, start, end int) (string, int, int) {
	start = min(max(start, 0), len(text))
	end = min(max(end, start), len(text))
	const limit = 320
	from, to := 0, len(text)
	if len(text) > limit {
		from = max(start-80, 0)
		for from > 0 && !utf8.RuneStart(text[from]) {
			from--
		}
		to = min(from+limit, len(text))
		for to < len(text) && to > from && !utf8.RuneStart(text[to]) {
			to--
		}
	}
	prefix, suffix := "", ""
	if from > 0 {
		prefix = "..."
	}
	if to < len(text) {
		suffix = "..."
	}
	lead := displayWidth(prefix + escapeC0(text[from:start]))
	finish := displayWidth(prefix + escapeC0(text[from:min(end, to)]))
	return prefix + escapeC0(text[from:to]) + suffix, lead, max(finish-lead, 1)
}

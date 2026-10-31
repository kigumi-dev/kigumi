package driver_test

import (
	"fmt"
	"strconv"
	"strings"
)

func parseRecipe(text string) (recipe, error) {
	var rec recipe
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		st, err := parseStep(line)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", line, err)
		}
		rec = append(rec, st)
	}
	return rec, nil
}

// words splits a line at spaces outside quoted text.
func words(line string) ([]string, error) {
	var out []string
	for line != "" {
		end := 0
		for end < len(line) && line[end] != ' ' {
			if line[end] == '"' {
				q, err := strconv.QuotedPrefix(line[end:])
				if err != nil {
					return nil, err
				}
				end += len(q)
				continue
			}
			end++
		}
		out = append(out, line[:end])
		line = strings.TrimPrefix(line[end:], " ")
	}
	return out, nil
}

func unquoteNth(w string) (text string, nth int, err error) {
	q, err := strconv.QuotedPrefix(w)
	if err != nil {
		return "", 0, err
	}
	text, err = strconv.Unquote(q)
	if rest := strings.TrimPrefix(w[len(q):], "#"); rest != "" {
		nth, err = strconv.Atoi(rest)
	}
	return text, nth, err
}

func parseStep(line string) (step, error) {
	ws, err := words(line)
	if err != nil {
		return step{}, err
	}
	if len(ws) < 4 {
		return step{}, fmt.Errorf("want <edit> <function> <site> <opcode>")
	}
	st := step{edit: ws[0], anchor: ws[3]}
	if st.fnName, st.fnNth, err = unquoteNth(ws[1]); err != nil {
		return step{}, err
	}
	blk, inst, ok := strings.Cut(strings.TrimPrefix(ws[2], "b"), "/")
	if !ok {
		return step{}, fmt.Errorf("site %q is not b<block>/<inst>", ws[2])
	}
	if st.blk, err = strconv.Atoi(blk); err != nil {
		return step{}, err
	}
	st.inst = -1
	if inst != "term" {
		if st.inst, err = strconv.Atoi(inst); err != nil {
			return step{}, err
		}
	}
	if len(ws) > 4 && ws[4] != "|" {
		return step{}, fmt.Errorf("choices start with |")
	}
	for _, w := range ws[min(len(ws), 5):] {
		pk, err := parsePick(w)
		if err != nil {
			return step{}, err
		}
		st.picks = append(st.picks, pk)
	}
	return st, nil
}

func parsePick(w string) (pick, error) {
	var err error
	pk := pick{kind: w[0]}
	body := w[1:]
	switch pk.kind {
	case 'i':
		pk.n, err = strconv.Atoi(body)
	case 'l':
		if rest, ok := strings.CutPrefix(body, "+"); ok {
			pk.oob = true
			pk.nth, err = strconv.Atoi(rest)
		} else {
			pk.text, pk.nth, err = unquoteNth(body)
		}
	case 't':
		pk.text, pk.nth, err = unquoteNth(body)
	case 'e':
		kind, rest, _ := strings.Cut(body, ":")
		k, ok := entityKindByName(kind)
		if !ok {
			return pick{}, fmt.Errorf("no entity kind %q", kind)
		}
		pk.ekind = k
		pk.text, pk.nth, err = unquoteNth(rest)
	case 's', 'p', 'v':
		pk.text, err = strconv.Unquote(body)
	default:
		err = fmt.Errorf("unknown choice %q", w)
	}
	return pk, err
}

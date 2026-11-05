package vm

import "bytes"

func shiftAscii(s []byte, lo, hi byte, by int) []byte {
	out := append([]byte{}, s...)
	for i, b := range out {
		if b >= lo && b <= hi {
			out[i] = byte(int(b) + by)
		}
	}
	return out
}

func charObjs(s []byte) []*obj {
	var out []*obj
	for i := 0; i < len(s); {
		w := min(utf8Width(s[i]), len(s)-i)
		point := rune(s[i])
		switch w {
		case 4:
			point &= 7
		case 3:
			point &= 15
		case 2:
			point &= 31
		}
		for k := 1; k < w; k++ {
			point = point<<6 | rune(s[i+k]&63)
		}
		out = append(out, mkChar(point))
		i += w
	}
	return out
}

func utf8Width(lead byte) int {
	switch {
	case lead >= 240:
		return 4
	case lead >= 224:
		return 3
	case lead >= 192:
		return 2
	}
	return 1
}

func (m *Machine) splitPieces(s, sep *obj) []*obj {
	if len(sep.str()) == 0 {
		return []*obj{retain(s)}
	}
	var out []*obj
	start := 0
	for i := 0; i+len(sep.str()) <= len(s.str()); {
		if !bytes.HasPrefix(s.str()[i:], sep.str()) {
			i++
			continue
		}
		out = append(out, mkStr(append([]byte{}, s.str()[start:i]...)))
		i += len(sep.str())
		start = i
	}
	return append(out, mkStr(append([]byte{}, s.str()[start:]...)))
}

func linePieces(s []byte) []*obj {
	var out []*obj
	for len(s) > 0 {
		if i := bytes.IndexByte(s, '\n'); i >= 0 {
			line := s[:i]
			if len(line) > 0 && line[len(line)-1] == 13 {
				line = line[:len(line)-1]
			}
			out = append(out, mkStr(append([]byte{}, line...)))
			s = s[i+1:]
			continue
		}
		// The unterminated last line keeps its CR; only a line ended by LF
		// had a CRLF ending to remove.
		out = append(out, mkStr(append([]byte{}, s...)))
		break
	}
	return out
}

func graphemePieces(s []byte) []*obj {
	var out []*obj
	for i := 0; i < len(s); {
		w := min(utf8Width(s[i]), len(s)-i)
		out = append(out, mkStr(append([]byte{}, s[i:i+w]...)))
		i += w
	}
	return out
}

package lsp

import "bytes"

type hunk struct{ a0, a1, b0, b1 int }

// maxEditDistance bounds the Myers search; a larger difference is reported as one hunk.
const maxEditDistance = 2000

func splitLines(src []byte) [][]byte {
	var lines [][]byte
	for len(src) > 0 {
		n := bytes.IndexByte(src, '\n') + 1
		if n == 0 {
			n = len(src)
		}
		lines = append(lines, src[:n])
		src = src[n:]
	}
	return lines
}

func diffLines(a, b [][]byte) []hunk {
	lo := 0
	for lo < len(a) && lo < len(b) && bytes.Equal(a[lo], b[lo]) {
		lo++
	}
	ea, eb := len(a), len(b)
	for ea > lo && eb > lo && bytes.Equal(a[ea-1], b[eb-1]) {
		ea--
		eb--
	}
	if lo == ea && lo == eb {
		return nil
	}
	if ea == lo || eb == lo {
		return []hunk{{lo, ea, lo, eb}}
	}
	ids := map[string]int32{}
	intern := func(lines [][]byte) []int32 {
		out := make([]int32, len(lines))
		for i, l := range lines {
			id, ok := ids[string(l)]
			if !ok {
				id = int32(len(ids))
				ids[string(l)] = id
			}
			out[i] = id
		}
		return out
	}
	del, ins, ok := myers(intern(a[lo:ea]), intern(b[lo:eb]))
	if !ok {
		return []hunk{{lo, ea, lo, eb}}
	}
	return hunksOf(del, ins, lo)
}

func myers(a, b []int32) (del, ins []bool, ok bool) {
	n, m := len(a), len(b)
	limit := min(n+m, maxEditDistance)
	off := limit + 1
	v := make([]int, 2*limit+3)
	var trace [][]int
	for d := 0; d <= limit; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				del, ins = backtrack(trace, n, m, d)
				return del, ins, true
			}
		}
		trace = append(trace, append([]int(nil), v[off-d-1:off+d+2]...))
	}
	return nil, nil, false
}

// backtrack reads the path off the saved frontiers; trace[d] holds v[-d-1..d+1] after round d.
func backtrack(trace [][]int, n, m, d int) (del, ins []bool) {
	del, ins = make([]bool, n), make([]bool, m)
	x, y := n, m
	for ; d > 0; d-- {
		prev := trace[d-1]
		at := func(k int) int { return prev[k+d] }
		k := x - y
		var pk int
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := at(pk)
		sx, sy := px, px-k
		if pk == k-1 {
			sx, sy = px+1, px-pk
		}
		for x > sx && y > sy {
			x--
			y--
		}
		if pk == k+1 {
			y--
			ins[y] = true
		} else {
			x--
			del[x] = true
		}
	}
	return del, ins
}

func hunksOf(del, ins []bool, base int) []hunk {
	var out []hunk
	i, j := 0, 0
	for i < len(del) || j < len(ins) {
		if i < len(del) && j < len(ins) && !del[i] && !ins[j] {
			i++
			j++
			continue
		}
		h := hunk{a0: base + i, b0: base + j}
		for i < len(del) && del[i] {
			i++
		}
		for j < len(ins) && ins[j] {
			j++
		}
		h.a1, h.b1 = base+i, base+j
		out = append(out, h)
	}
	return out
}

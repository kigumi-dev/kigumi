package syntax_test

import (
	"strings"
	"testing"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

const benchUnit = `
pub type Point%d = {
    pub x Int
    pub y Int
}

pub fn Point%d.len(self) -> Int {
    let dx = self.x
    let dy = self.y
    dx * dx + dy * dy
}

fn classify%d(n: Int) -> String {
    match n {
        0 -> "zero"
        _ -> if n < 0 {
            "negative"
        } else {
            "positive"
        }
    }
}

fn sum%d(xs: Array[Int]) -> Int {
    let mut total = 0
    for x in xs {
        total = total + x
    }
    total
}

fn makeAdder%d(n: Int) -> fn(Int) -> Int {
    (m: Int) => m + n
}
`

func genSource(n int) string {
	var sb strings.Builder
	for i := 0; sb.Len() < n; i++ {
		sb.WriteString(strings.ReplaceAll(benchUnit, "%d", itoa(i)))
	}
	return sb.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func benchmarkParse(b *testing.B, size int) {
	src := []byte(genSource(size))
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		tree := syntax.Parse(token.NewFile("bench.kg", src))
		if tree.HasErrors() {
			for _, d := range tree.Diags {
				b.Errorf("%s", d.Msg)
			}
			b.FailNow()
		}
	}
}

// BenchmarkParse10KiB and BenchmarkParse1MiB parse representative source
// at two sizes, so an O(n^2) pass or a cache effect past a few KiB shows up.
func BenchmarkParse10KiB(b *testing.B) { benchmarkParse(b, 10*1024) }
func BenchmarkParse1MiB(b *testing.B)  { benchmarkParse(b, 1024*1024) }

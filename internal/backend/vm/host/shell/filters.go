package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// The filters below mirror std/shell/filters.kg byte for byte: lines split at
// "\n", every emitted line is newline terminated, an unterminated last line
// still counts, and no utility decodes UTF-8 unless its count says so.

func readInput(in io.Reader) ([]byte, error) {
	return io.ReadAll(in)
}

func splitLines(in []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := range in {
		if in[i] == '\n' {
			out = append(out, in[start:i])
			start = i + 1
		}
	}
	if start < len(in) {
		out = append(out, in[start:])
	}
	return out
}

func writeLines(out io.Writer, lines [][]byte) error {
	var buf bytes.Buffer
	for _, line := range lines {
		buf.Write(line)
		buf.WriteByte('\n')
	}
	_, err := out.Write(buf.Bytes())
	return err
}

// CatFilter copies the input through unchanged.
func CatFilter() func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	}
}

// HeadFilter keeps the first count lines.
func HeadFilter(count int) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		lines := splitLines(data)
		if count < len(lines) {
			lines = lines[:count]
		}
		return writeLines(out, lines)
	}
}

// TailFilter keeps the last count lines.
func TailFilter(count int) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		lines := splitLines(data)
		if count < len(lines) {
			lines = lines[len(lines)-count:]
		}
		return writeLines(out, lines)
	}
}

func isWordSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

func countWords(in []byte) int {
	count := 0
	inWord := false
	for _, b := range in {
		if isWordSpace(b) {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}

// countChars counts bytes that are not UTF-8 continuation bytes, which stays
// defined for input that is not valid UTF-8.
func countChars(in []byte) int {
	count := 0
	for _, b := range in {
		if b&0xC0 != 0x80 {
			count++
		}
	}
	return count
}

func countLines(in []byte) int {
	count := 0
	for _, b := range in {
		if b == '\n' {
			count++
		}
	}
	if len(in) > 0 && in[len(in)-1] != '\n' {
		count++
	}
	return count
}

// WcFilter prints the counts options select, in the order lines, words, bytes, chars.
func WcFilter(lines, words, size, chars bool) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		var buf []byte
		add := func(n int) {
			if len(buf) > 0 {
				buf = append(buf, ' ')
			}
			buf = strconv.AppendInt(buf, int64(n), 10)
		}
		if lines {
			add(countLines(data))
		}
		if words {
			add(countWords(data))
		}
		if size {
			add(len(data))
		}
		if chars {
			add(countChars(data))
		}
		buf = append(buf, '\n')
		_, err = out.Write(buf)
		return err
	}
}

// SortFilter orders lines by byte value, optionally reversed and deduplicated.
func SortFilter(reverse, unique bool) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		lines := splitLines(data)
		sort.SliceStable(lines, func(i, j int) bool {
			order := bytes.Compare(lines[i], lines[j])
			if reverse {
				return order > 0
			}
			return order < 0
		})
		if unique {
			distinct := lines[:0]
			for i, line := range lines {
				if i == 0 || !bytes.Equal(distinct[len(distinct)-1], line) {
					distinct = append(distinct, line)
				}
			}
			lines = distinct
		}
		return writeLines(out, lines)
	}
}

// UniqFilter keeps the runs of equal adjacent lines that the options select.
func UniqFilter(count, repeated, unique bool) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		lines := splitLines(data)
		var buf bytes.Buffer
		for i := 0; i < len(lines); {
			j := i + 1
			for j < len(lines) && bytes.Equal(lines[i], lines[j]) {
				j++
			}
			run := j - i
			keep := true
			if repeated {
				keep = run > 1
			} else if unique {
				keep = run == 1
			}
			if keep {
				if count {
					buf.WriteString(strconv.Itoa(run))
					buf.WriteByte(' ')
				}
				buf.Write(lines[i])
				buf.WriteByte('\n')
			}
			i = j
		}
		_, err = out.Write(buf.Bytes())
		return err
	}
}

// CutFilter selects 1-based fields of every line. A line without the
// delimiter is copied through, as the line-oriented tools mirror do.
func CutFilter(delim []byte, fields []int64) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		for _, line := range splitLines(data) {
			if !bytes.Contains(line, delim) {
				buf.Write(line)
				buf.WriteByte('\n')
				continue
			}
			pieces := bytes.Split(line, delim)
			printed := false
			for _, field := range fields {
				if field < 1 || field > int64(len(pieces)) {
					continue
				}
				if printed {
					buf.Write(delim)
				}
				buf.Write(pieces[field-1])
				printed = true
			}
			buf.WriteByte('\n')
		}
		_, err = out.Write(buf.Bytes())
		return err
	}
}

// TrFilter maps from to to byte by byte, optionally deleting or squeezing.
func TrFilter(from, to []byte, squeeze, delete bool) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		squeezeSet := to
		if len(squeezeSet) == 0 {
			squeezeSet = from
		}
		inSet := func(set []byte, b byte) int {
			for i := range set {
				if set[i] == b {
					return i
				}
			}
			return -1
		}
		var buf bytes.Buffer
		last := -1
		for _, b := range data {
			at := inSet(from, b)
			if delete && at >= 0 {
				continue
			}
			translated := b
			if at >= 0 && len(to) > 0 {
				pick := at
				if pick >= len(to) {
					pick = len(to) - 1
				}
				translated = to[pick]
			}
			if squeeze && last == int(translated) && inSet(squeezeSet, translated) >= 0 {
				continue
			}
			buf.WriteByte(translated)
			last = int(translated)
		}
		_, err = out.Write(buf.Bytes())
		return err
	}
}

// TeeFilter writes the input to path and passes the same bytes on. dir is
// the run's working directory, so a relative path resolves like a redirect.
func TeeFilter(path string, append bool, dir string) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		data, err := readInput(in)
		if err != nil {
			return err
		}
		target := path
		if dir != "" && !filepath.IsAbs(target) {
			target = filepath.Join(dir, target)
		}
		flags := os.O_WRONLY | os.O_CREATE
		if append {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(target, flags, 0o644)
		if err != nil {
			return fmt.Errorf("cannot open %s: %s", path, OSErrText(err))
		}
		if _, werr := f.Write(data); werr != nil {
			f.Close()
			return werr
		}
		if cerr := f.Close(); cerr != nil {
			return cerr
		}
		_, err = out.Write(data)
		return err
	}
}

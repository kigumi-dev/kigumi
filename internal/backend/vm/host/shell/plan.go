// Package shell runs shell plans: external commands chained
// through pipes, in-process filters and redirects. Both executors hand it
// the plan std/shell parsed.
package shell

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"kigumi/internal/compiler/syntax"
)

// Plan is a shell pipeline: external commands and in-process stages.
type Plan struct {
	Stages []Stage
}

type Stage struct {
	Argv   []string
	Filter func(in io.Reader, out io.Writer) error
	Redirs []Redirect
}

type Redirect struct {
	Op     uint32
	Fd     int
	Target string
}

// GrepFilter keeps the lines matching pattern, byte by byte like the native
// runtime: lines split at "\n" only, and a line holding a NUL never matches.
func GrepFilter(pattern string) func(io.Reader, io.Writer) error {
	return func(in io.Reader, out io.Writer) error {
		re, err := regexp.Compile(byteRunes([]byte(pattern)))
		if err != nil {
			return err
		}
		br := bufio.NewReader(in)
		for {
			line, rerr := br.ReadBytes('\n')
			if len(line) > 0 {
				line = bytes.TrimSuffix(line, []byte("\n"))
				if !bytes.Contains(line, []byte{0}) && re.MatchString(byteRunes(line)) {
					if _, werr := out.Write(append(line, '\n')); werr != nil {
						return werr
					}
				}
			}
			if rerr == io.EOF {
				return nil
			}
			if rerr != nil {
				return rerr
			}
		}
	}
}

func byteRunes(b []byte) string {
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

// fdTable is a stage's file descriptors 0, 1 and 2 while its redirects are
// applied. Entries stay io.Reader/io.Writer rather than *os.File so an
// unredirected fd can keep pointing at the pipeline's own endpoint, and so
// `>&` copies whatever the descriptor holds at that point in source order.
type fdTable [3]any

// applyRedirects walks a stage's redirects left to right, which is what makes
// `2>&1 >out` differ from `>out 2>&1`. Files it opens are returned so
// the parent can close its own copies once the child has them.
func applyRedirects(fds *fdTable, redirs []Redirect, dir string) ([]*os.File, error) {
	var opened []*os.File
	for _, r := range redirs {
		if r.Fd < 0 || r.Fd >= len(fds) {
			return opened, fmt.Errorf("only file descriptors 0, 1 and 2 can be redirected, got %d", r.Fd)
		}
		if r.Op == syntax.RedirDupIn || r.Op == syntax.RedirDupOut {
			n, err := strconv.Atoi(r.Target)
			if err != nil || n < 0 || n >= len(fds) {
				return opened, fmt.Errorf("`%s` is not a file descriptor that can be duplicated", r.Target)
			}
			fds[r.Fd] = fds[n]
			continue
		}
		if strings.IndexByte(r.Target, 0) >= 0 {
			return opened, errors.New("cannot open redirect target: it contains a NUL byte")
		}
		f, err := openRedirect(r, dir)
		if err != nil {
			return opened, fmt.Errorf("cannot open %s: %s", r.Target, OSErrText(err))
		}
		opened = append(opened, f)
		fds[r.Fd] = f
	}
	return opened, nil
}

func openRedirect(r Redirect, dir string) (*os.File, error) {
	path := r.Target
	if dir != "" && !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	switch r.Op {
	case syntax.RedirIn:
		return os.Open(path)
	case syntax.RedirAppend:
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	default:
		return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	}
}

func asReader(v any) io.Reader {
	r, _ := v.(io.Reader)
	return r
}

func asWriter(v any) io.Writer {
	w, _ := v.(io.Writer)
	return w
}

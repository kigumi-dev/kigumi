package driver

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

func isZig(cc []string) bool {
	return len(cc) > 0 && (strings.HasSuffix(cc[0], "zig") || filepath.Base(cc[0]) == "zig")
}

// hostLink is one hosted executable's compile-and-link step; args are the compile flags every input shares.
type hostLink struct {
	m              *Module
	cc, args       []string
	inputs         []string
	dir            string
	opts           BuildOptions
	t              Target
	link           Linkage
	cflags, lflags []string
	stderr         io.Writer
}

// run links h into staged. zig links libm itself and only for the symbols the program references, so -lm goes only to another compiler.
func (h hostLink) run(staged string) error {
	m, cc, opts, t := h.m, h.cc, h.opts, h.t
	linkOut, linkInputsList, err := linkInputs(m.compiler, opts.pinCompDir, h.dir, staged, h.inputs)
	if err != nil {
		return err
	}
	linkArgs := append(append([]string{}, h.args...), "-o", linkOut)
	linkArgs = append(linkArgs, linkInputsList...)
	if opts.pinCompDir {
		linkArgs = append(linkArgs, "-fdebug-compilation-dir=.")
		linkArgs = append(linkArgs, prefixMapFlag(m.compiler)...)
	}
	if sectionGC(t) {
		linkArgs = append(linkArgs, "-Wl,--gc-sections")
	}
	switch {
	case opts.SizeMin:
		linkArgs = append(linkArgs, "-s")
	case h.link.Static:
		// zig's static libc and compiler-rt objects record absolute cache and work paths in their DWARF; zig strips symbols with it.
		linkArgs = append(linkArgs, "-Wl,--strip-debug")
	}
	if !isZig(cc) {
		linkArgs = append(linkArgs, "-lm")
	}
	if _, ok := m.Packages["std/dl"]; ok && !h.link.Static {
		linkArgs = append(linkArgs, "-ldl")
	}
	for _, l := range m.Links {
		if l.Search != "" {
			// l.Search may be relative to the compiler Cwd; resolve now since -L is read against cmd.Dir, which pinCompDir moves to dir.
			search := l.Search
			if abs, err := m.compiler.Abs(search); err == nil {
				search = abs
			}
			linkArgs = append(linkArgs, "-L"+search)
		}
		if l.Library != "" {
			linkArgs = append(linkArgs, "-l"+l.Library)
		}
	}
	if opts.LinkerScript != "" {
		script := opts.LinkerScript
		if abs, err := m.compiler.Abs(script); err == nil {
			script = abs
		}
		linkArgs = append(linkArgs, "-T", script)
	}
	if opts.Entry != "" {
		linkArgs = append(linkArgs, "-Wl,--entry="+opts.Entry)
	}
	linkArgs = append(linkArgs, h.lflags...)
	linkArgs = append(linkArgs, h.cflags...)
	cmd := exec.Command(cc[0], linkArgs...)
	var diag bytes.Buffer
	cmd.Stderr = io.MultiWriter(h.stderr, &diag)
	if opts.pinCompDir {
		cmd.Dir = h.dir
	}
	if opts.ccEnv != nil {
		cmd.Env = opts.ccEnv
	}
	if err := cmd.Run(); err != nil {
		if h.link.Static && strings.Contains(diag.String(), "undefined symbol") {
			fix := "--link dynamic links the host C library"
			if opts.Target.Triple != "" {
				fix = "a -gnu target links glibc"
			}
			return fmt.Errorf("link failed: %w; the static musl C library lacks a symbol named above, and %s", err, fix)
		}
		return fmt.Errorf("link failed: %w", err)
	}
	return nil
}

package driver

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kigumi/internal/llgen"
)

// buildOnce is BuildWith's single-build implementation; buildReproducible
// calls it twice with pinCompDir and ccEnv set.
func buildOnce(m *Module, out string, stderr io.Writer, opts BuildOptions) (bool, error) {
	t := opts.Target
	if t.OS == "" {
		t = HostTarget()
	}
	code, ok, err := compile(m, stderr, emitOptionsFor(t))
	if err != nil || !ok {
		return ok, err
	}
	cc := CCompiler()
	if cc == nil {
		return false, fmt.Errorf("no C compiler found (need zig or clang)")
	}
	dir, err := os.MkdirTemp("", "kigumi-build")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	ll := filepath.Join(dir, "module.ll")
	if err := os.WriteFile(ll, []byte(code.IR), 0o644); err != nil {
		return false, err
	}
	inputs := []string{ll}
	if code.CShim != "" {
		shim := filepath.Join(dir, "kigumi_shim.c")
		if err := os.WriteFile(shim, []byte(code.CShim), 0o644); err != nil {
			return false, err
		}
		inputs = append(inputs, shim)
	}
	for _, f := range llgen.RuntimeFiles(t.Sys, t.OS) {
		path := filepath.Join(dir, f.Name)
		if err := os.WriteFile(path, []byte(f.Src), 0o644); err != nil {
			return false, err
		}
		if strings.HasSuffix(f.Name, ".c") {
			inputs = append(inputs, path)
		}
	}
	inputs = append(inputs, m.Sources...)
	// llgen emits no target triple, so the compiler always "overrides" one
	// from the host; only that expected warning is silenced.
	args := append(cc[1:], "-Wno-override-module", "-O1")
	if t.compileTriple() != "" {
		args = append(args, "-target", t.compileTriple())
	}
	if t.Freestanding() {
		args = append(args, "-DKIGUMI_FREESTANDING", "-nostdlib")
	}
	if t.COFF() {
		// No CRT to supply __chkstk; raise its threshold out of reach instead.
		args = append(args, "-DKIGUMI_COFF", "-mstack-probe-size=1048576")
	}
	if t.Bare() {
		compileFlags, _ := splitArchiveCFlags(opts.CFlags)
		args = append(args, compileFlags...)
		reportStaleFingerprint(out, t, stderr)
		ok, err := buildArchive(cc, args, inputs, dir, out, opts.ccEnv, stderr)
		if err != nil || !ok {
			return ok, err
		}
		if err := WriteFingerprintSidecar(out, CurrentFingerprint(t)); err != nil {
			return false, err
		}
		err = auditBareArchive(cc, out)
		return err == nil, err
	}
	return publishBuildOutput(out, func(staged string) error {
		linkOut, linkInputsList, err := linkInputs(opts.pinCompDir, dir, staged, inputs)
		if err != nil {
			return err
		}
		linkArgs := append(append([]string{}, args...), "-o", linkOut)
		linkArgs = append(linkArgs, linkInputsList...)
		if opts.pinCompDir {
			linkArgs = append(linkArgs, "-fdebug-compilation-dir=.")
		}
		linkArgs = append(linkArgs, "-lm")
		if _, ok := m.Packages["std/dl"]; ok {
			linkArgs = append(linkArgs, "-ldl")
		}
		for _, l := range m.Links {
			if l.Search != "" {
				// l.Search may be relative to the process cwd; resolve now since -L is read against cmd.Dir, which pinCompDir moves to dir.
				search := l.Search
				if abs, err := filepath.Abs(search); err == nil {
					search = abs
				}
				linkArgs = append(linkArgs, "-L"+search)
			}
			if l.Library != "" {
				linkArgs = append(linkArgs, "-l"+l.Library)
			}
		}
		if opts.LinkerScript != "" {
			linkArgs = append(linkArgs, "-T", opts.LinkerScript)
		}
		if opts.Entry != "" {
			linkArgs = append(linkArgs, "-Wl,--entry="+opts.Entry)
		}
		linkArgs = append(linkArgs, opts.LinkFlags...)
		linkArgs = append(linkArgs, opts.CFlags...)
		cmd := exec.Command(cc[0], linkArgs...)
		cmd.Stderr = stderr
		if opts.pinCompDir {
			cmd.Dir = dir
		}
		if opts.ccEnv != nil {
			cmd.Env = opts.ccEnv
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("link failed: %w", err)
		}
		return nil
	})
}

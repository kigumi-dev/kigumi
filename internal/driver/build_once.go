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

// wasi-libc's setjmp.h needs exception handling and a wasm backend has no jump of its own, so wasm keeps task panics fatal.
func wantsTaskCatch(t Target) bool {
	if t.OS == "wasi" {
		return false
	}
	return !t.Bare() || bareJumpTarget(t)
}

// Only the x86 backends lower __builtin_setjmp, the one jump that needs no libc.
func bareJumpTarget(t Target) bool {
	return t.Bare() && (t.Arch == "amd64" || t.Arch == "386")
}

// buildOnce is BuildWith's single-build implementation; buildReproducible
// calls it twice with pinCompDir and ccEnv set.
func buildOnce(m *Module, out string, stderr io.Writer, opts BuildOptions) (bool, error) {
	t := opts.Target
	if t.OS == "" {
		t = m.compiler.HostOrAmbient()
	}
	code, ok, err := compile(m, stderr, t, emitOptionsFor(t))
	if err != nil || !ok {
		return ok, err
	}
	code = withRuntimeABI(t, code, opts.CFlags)
	cflags, err := resolveFlagPaths(m.compiler, opts.CFlags)
	if err != nil {
		return false, err
	}
	lflags, err := resolveFlagPaths(m.compiler, opts.LinkFlags)
	if err != nil {
		return false, err
	}
	if err := checkLinkedRuntimeABI(m, cflags, lflags, code.Abi); err != nil {
		return false, err
	}
	cc, err := CCompilerFor(m)
	if err != nil {
		return false, err
	}
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
	// -O1 keeps zig cc out of Debug mode, which instruments C with UBSan calls freestanding has no runtime for.
	args := append(cc[1:], "-Wno-override-module", "-O1")
	if t.compileTriple() != "" {
		args = append(args, "-target", t.compileTriple())
	}
	if t.Freestanding() {
		args = append(args, "-nostdlib")
	}
	if t.Bare() && t.Arch == "arm" {
		args = append(args, "-fno-unwind-tables", "-fno-asynchronous-unwind-tables")
	}
	for _, def := range runtimeDefines(t, code) {
		args = append(args, "-D"+def)
	}
	args = append(args, abiDefine(code.Abi))
	if t.COFF() {
		// No CRT to supply __chkstk; raise its threshold out of reach instead.
		args = append(args, "-mstack-probe-size=1048576")
	}
	if t.Bare() {
		compileFlags, _ := splitArchiveCFlags(cflags)
		args = append(args, compileFlags...)
		if t.Arch != "wasm32" {
			// A wasm object compiled position-independent imports __memory_base and __table_base from the linker.
			args = append(args, "-fPIE")
		}
		fp := CurrentFingerprint(t, ZigVersionOf(cc))
		fp.RuntimeAbi = code.Abi
		reportStaleFingerprint(out, fp, stderr)
		ok, err := publishBuildOutput(out, func(staged string) error {
			built, err := buildArchive(m.compiler, cc, args, inputs, dir, staged, opts.ccEnv, stderr)
			if err != nil || !built {
				return err
			}
			return auditBareArchive(m.compiler, cc, staged, out, m.Manifest.Embedder)
		})
		if err != nil || !ok {
			return ok, err
		}
		withProvenance(&fp, m, t, code)
		return true, WriteFingerprintSidecar(out, fp)
	}
	ok, err = publishBuildOutput(out, func(staged string) error {
		linkOut, linkInputsList, err := linkInputs(m.compiler, opts.pinCompDir, dir, staged, inputs)
		if err != nil {
			return err
		}
		linkArgs := append(append([]string{}, args...), "-o", linkOut)
		linkArgs = append(linkArgs, linkInputsList...)
		if opts.pinCompDir {
			linkArgs = append(linkArgs, "-fdebug-compilation-dir=.")
			linkArgs = append(linkArgs, prefixMapFlag(m.compiler)...)
		}
		linkArgs = append(linkArgs, "-lm")
		if _, ok := m.Packages["std/dl"]; ok {
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
		linkArgs = append(linkArgs, lflags...)
		linkArgs = append(linkArgs, cflags...)
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
	if err != nil || !ok {
		return ok, err
	}
	fp := CurrentFingerprint(t, ZigVersionOf(cc))
	withProvenance(&fp, m, t, code)
	return true, WriteFingerprintSidecar(out, fp)
}

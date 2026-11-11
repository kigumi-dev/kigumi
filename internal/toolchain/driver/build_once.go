package driver

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/native/audit"
	"kigumi/internal/toolchain/native/link"
)

// wasi-libc's setjmp.h needs exception handling and a wasm backend has no jump of its own, so wasm keeps task panics fatal.
func wantsTaskCatch(t buildconfig.Target) bool {
	if t.OS == "wasi" {
		return false
	}
	return !t.Bare() || bareJumpTarget(t)
}

// Only the x86 backends lower __builtin_setjmp, the one jump that needs no libc.
func bareJumpTarget(t buildconfig.Target) bool {
	return t.Bare() && (t.Arch == "amd64" || t.Arch == "386")
}

// buildOnce is BuildWith's single-build implementation; buildReproducible
// calls it twice with pinCompDir and ccEnv set.
func buildOnce(m *Module, out string, stderr io.Writer, opts BuildOptions) (bool, error) {
	l, ok, err := compileLinked(m, stderr, opts, false)
	if err != nil || !ok {
		return ok, err
	}
	m, code, t, linkage, cc, cflags, lflags := l.m, l.code, l.t, l.link, l.cc, l.cflags, l.lflags
	if opts.Report != nil {
		*opts.Report = BuildReport{Linkage: linkage, SizeMin: opts.SizeMin, SectionGC: sectionGC(t), LTO: lto(t), Archive: t.Bare()}
	}
	dir, err := os.MkdirTemp("", "kigumi-build")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	compiler := m.compiler
	if opts.pinCompDir {
		compiler.PrefixMaps = append(reproduciblePrefixMaps(m, dir), compiler.PrefixMaps...)
	}
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
	for _, f := range llgen.RuntimeZigFiles() {
		path := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return false, err
		}
		if err := os.WriteFile(path, []byte(f.Src), 0o644); err != nil {
			return false, err
		}
	}
	zrt, err := compileZigRuntime(m, cc, t, dir, opts.ccEnv, stderr)
	if err != nil {
		return false, err
	}
	inputs = append(inputs, zrt.object)
	inputs = append(inputs, m.Sources...)
	args := append(append(cc[1:], "-Wno-override-module"), codegenFlags(t, opts.SizeMin)...)
	if t.CompileTriple() != "" {
		args = append(args, "-target", t.CompileTriple())
	}
	if t.Freestanding() {
		args = append(args, "-nostdlib")
	}
	if sectionGC(t) {
		args = append(args, "-ffunction-sections", "-fdata-sections")
	}
	for _, def := range append(runtimeDefines(t, code), code.StdOps...) {
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
		fp := CurrentFingerprint(t, zigRuntimeVersion(cc, zrt))
		withProvenance(&fp, m, t, code)
		artifact.ReportStale(out, fp, stderr)
		return artifact.PublishWithFingerprint(out, fp, func(staged string) error {
			built, err := buildArchive(compiler, cc, args, inputs, dir, staged, opts.ccEnv, stderr, opts.pinCompDir && t.COFF())
			if err != nil || !built {
				return err
			}
			ar, err := link.Archiver(m.compiler, cc)
			if err != nil {
				return err
			}
			return audit.Archive(m.compiler, ar, staged, out, llgen.FreestandingSymbols(), m.Manifest.Embedder)
		})
	}
	fp := CurrentFingerprint(t, zigRuntimeVersion(cc, zrt))
	withProvenance(&fp, m, t, code)
	return artifact.PublishWithFingerprint(out, fp, func(staged string) error {
		return hostLink{m: m, compiler: compiler, cc: cc, args: args, inputs: inputs, dir: dir, opts: opts, t: t, link: linkage, cflags: cflags, lflags: lflags, stderr: stderr}.run(staged)
	})
}

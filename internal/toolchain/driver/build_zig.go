package driver

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sort"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/native/link"
	"kigumi/internal/toolchain/zig/store"
)

// zigRuntime is the compiled Zig translation unit of the runtime: its object
// path and the zig that produced it (for the artifact fingerprint).
type zigRuntime struct {
	object, compiler string
}

// zigCompilerFor picks the zig that compiles the runtime's Zig module: the C
// compiler when it is zig itself, else the module's pinned/on-PATH zig. A
// clang-only toolchain has no way to compile Zig source, so it is an error
// rather than a silent fallback to a second C implementation.
func zigCompilerFor(m *Module, cc []string) (string, error) {
	if link.IsZig(cc) {
		return cc[0], nil
	}
	path, err := m.ResolveZig()
	if err != nil {
		return "", err
	}
	if path != "" {
		return path, nil
	}
	if path, err := m.compiler.LookTool("zig"); err == nil {
		return path, nil
	}
	if path := m.installedZig(); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("the runtime's float module is Zig source and needs a zig compiler; install zig or pin it in the manifest")
}

// installedZig returns the newest zig in the managed store. An unpinned
// module does not need one specific version, but a test or toolchain that
// installed zig in the store should not also have to put it on PATH.
func (m *Module) installedZig() string {
	root, err := m.compiler.ZigStoreRoot(m.zigHome)
	if err != nil || root == "" {
		return ""
	}
	versions, err := store.List(root)
	if err != nil || len(versions) == 0 {
		return ""
	}
	sort.Slice(versions, func(i, j int) bool {
		return module.CompareVersions(versions[i], versions[j]) < 0
	})
	path, _ := store.Installed(root, versions[len(versions)-1])
	return path
}

// compileZigRuntime builds zig/rt_float.zig (already written under dir) into
// one object for target t. ReleaseSafe keeps bounds and overflow checks on, so
// a broken bignum or digit-buffer invariant traps deterministically instead of
// corrupting the stack, which is what the port is for. -fno-stack-check matches
// the C runtime objects (which carry no probes either) and keeps freestanding
// archives from referencing compiler-rt's __zig_probe_stack. -fstrip drops the
// DWARF that would otherwise embed the temp directory and break --reproducible
// builds. A position-independent object is not requested for wasm32, where it
// would import __memory_base/__table_base (the C compile path skips -fPIE for
// the same reason).
func compileZigRuntime(m *Module, cc []string, t buildconfig.Target, dir string, env []string, stderr io.Writer) (zigRuntime, error) {
	zig, err := zigCompilerFor(m, cc)
	if err != nil {
		return zigRuntime{}, err
	}
	out := filepath.Join(dir, "rt_float.o")
	args := []string{"build-obj", "-OReleaseSafe", "-fno-compiler-rt", "-fno-stack-check", "-fstrip", "-ffunction-sections", "-fdata-sections", "-femit-bin=" + out}
	if t.Arch != "wasm32" {
		args = append(args, "-fPIC")
	}
	if t.Bare() {
		// A bare target has no unwinder, and arm's table entries need
		// compiler-rt's __aeabi_unwind_cpp_pr* helpers; the C compile path
		// passes the same pair for bare arm.
		args = append(args, "-fno-unwind-tables")
	}
	if triple := t.CompileTriple(); triple != "" {
		args = append(args, "-target", triple)
	}
	args = append(args, filepath.Join(dir, "zig", "rt_float.zig"))
	cmd := exec.Command(zig, args...)
	cmd.Dir = dir
	cmd.Stderr = stderr
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Run(); err != nil {
		return zigRuntime{}, fmt.Errorf("compile the Zig float runtime: %w", err)
	}
	return zigRuntime{object: out, compiler: zig}, nil
}

// zigRuntimeVersion is the zig version the fingerprint records: the C
// compiler's when it is zig, else the zig that built the float object.
func zigRuntimeVersion(cc []string, zrt zigRuntime) string {
	if v := link.ZigVersionOf(cc); v != "" {
		return v
	}
	return link.ZigVersionOf([]string{zrt.compiler})
}

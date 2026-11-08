package driver

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"kigumi/internal/toolchain/artifact"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/native/link"
	"kigumi/internal/toolchain/native/object"
)

// splitArchiveCFlags keeps linker-shaped flags out of buildArchive's compile-only step; zig cc corrupts objects when --gc-sections reaches a -c invocation.
func splitArchiveCFlags(flags []string) (compile, linkerLike []string) {
	for _, f := range flags {
		if strings.HasPrefix(f, "-Wl,") || strings.HasPrefix(f, "-l") || strings.HasPrefix(f, "-L") || strings.HasPrefix(f, "-T") {
			linkerLike = append(linkerLike, f)
			continue
		}
		compile = append(compile, f)
	}
	return compile, linkerLike
}

// buildArchive compiles each input to an object for the embedder to link (freestanding has no libc); dir/comp-dir is pinned so two builds don't diverge, and -ffunction/data-sections keeps the embedder's --gc-sections effective.
func buildArchive(o buildconfig.CompilerOptions, cc, args, inputs []string, dir, out string, env []string, stderr io.Writer, stripDebug bool) (bool, error) {
	var objs []string
	for i, in := range inputs {
		rel, err := link.PinnedInput(o, dir, in)
		if err != nil {
			return false, err
		}
		obj := fmt.Sprintf("obj%d.o", i)
		cargs := append(append([]string{}, args...), "-ffreestanding", "-fno-stack-protector", "-ffunction-sections", "-fdata-sections", "-fdebug-compilation-dir=.")
		cargs = append(append(cargs, link.PrefixMapFlag(o)...), "-c", "-o", obj, rel)
		cmd := exec.Command(cc[0], cargs...)
		cmd.Dir = dir
		cmd.Stderr = stderr
		if env != nil {
			cmd.Env = env
		}
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("compile %s: %w", filepath.Base(in), err)
		}
		if stripDebug {
			// CodeView names zig's temp cache object, so only an object without its debug sections is
			// byte-stable; compiling it as the plain build does keeps every other byte identical.
			if err := object.StripCOFFDebug(filepath.Join(dir, obj)); err != nil {
				return false, err
			}
		}
		objs = append(objs, obj)
	}
	ar, err := link.Archiver(o, cc)
	if err != nil {
		return false, err
	}
	return artifact.Publish(out, func(staged string) error {
		absStaged, err := o.Abs(staged)
		if err != nil {
			return err
		}
		// rcsD: D zeroes member timestamp/uid/gid, so archives of the same
		// objects are byte-identical across runs and machines.
		cmd := exec.Command(ar[0], append(append(ar[1:], "rcsD", absStaged), objs...)...)
		cmd.Dir = dir
		cmd.Stderr = stderr
		if env != nil {
			cmd.Env = env
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("archive failed: %w", err)
		}
		return nil
	})
}

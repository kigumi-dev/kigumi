package driver

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
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

// arTool is the archiver for cc: zig's own, since a plain system `ar`
// cannot read object files zig cc produced for some targets.
func arTool(o CompilerOptions, cc []string) ([]string, error) {
	if strings.HasSuffix(cc[0], "zig") || filepath.Base(cc[0]) == "zig" {
		return []string{cc[0], "ar"}, nil
	}
	ar, err := o.LookTool("ar")
	if err != nil {
		return nil, err
	}
	return []string{ar}, nil
}

// buildArchive compiles each input to an object for the embedder to link (freestanding has no libc); dir/comp-dir is pinned so two builds don't diverge, and -ffunction/data-sections keeps the embedder's --gc-sections effective.
func buildArchive(o CompilerOptions, cc, args, inputs []string, dir, out string, env []string, stderr io.Writer) (bool, error) {
	var objs []string
	for i, in := range inputs {
		// in may be relative; filepath.Rel errors if only one argument is absolute, and dir always is.
		absIn, err := o.Abs(in)
		if err != nil {
			return false, err
		}
		rel, err := filepath.Rel(dir, absIn)
		if err != nil {
			return false, err
		}
		obj := fmt.Sprintf("obj%d.o", i)
		cargs := append(append([]string{}, args...), "-ffreestanding", "-fno-stack-protector", "-ffunction-sections", "-fdata-sections", "-fdebug-compilation-dir=.", "-c", "-o", obj, rel)
		cmd := exec.Command(cc[0], cargs...)
		cmd.Dir = dir
		cmd.Stderr = stderr
		if env != nil {
			cmd.Env = env
		}
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("compile %s: %w", filepath.Base(in), err)
		}
		objs = append(objs, obj)
	}
	ar, err := arTool(o, cc)
	if err != nil {
		return false, err
	}
	return publishBuildOutput(out, func(staged string) error {
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

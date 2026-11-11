package driver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"kigumi/internal/toolchain/artifact"
)

// ExecOptions is what the CLI resolved for one build: secret files by
// declared name and the extra C flags for the artifacts.
type ExecOptions struct {
	Secrets map[string]string
	CFlags  []string
	// ToolEnv is the whole environment of a tool process; the driver's own env is never inherited, so the CLI passes an allowlist like PATH.
	ToolEnv []string
	// Reproducible builds every artifact with BuildOptions.Reproducible,
	// for the --reproducible flag.
	Reproducible bool
	// Env is the allowlisted compiler environment (shared.CCEnv) each
	// artifact's Reproducible build runs its two internal builds in.
	Env     []string
	Link    LinkMode
	SizeMin bool
	// Reports, when set, receives each artifact's BuildReport by artifact name.
	Reports map[string]BuildReport
}

func ExecuteGraph(p *BuildProgram, rg *ResolvedGraph, outDir string, opts ExecOptions, stderr io.Writer) error {
	outDir, err := p.Module.compiler.Abs(outDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	root, _ := p.Module.compiler.Abs(p.Module.Root)
	roots, err := p.Roots(outDir)
	if err != nil {
		return err
	}
	g := rg.Graph
	// The action cache lives under the build's own output directory, so the
	// tool environment needs no HOME and a later build into the same
	// directory can reuse an unchanged tool's outputs.
	cache := newActionCache(filepath.Join(outDir, actionCacheDirName))
	for _, f := range g.Files {
		// Re-check literal files at execution: the tree may have changed
		// since configure ran, and a symlink must not lead outside.
		if f.Kind == "literal" && !withinSandbox(roots, filepath.Join(root, filepath.FromSlash(f.Path))) {
			return fmt.Errorf("file %s is outside the module, its dependencies and the build output directory", f.Path)
		}
	}
	filePath := func(id int) string {
		f := g.Files[id]
		switch f.Kind {
		case "literal":
			return filepath.Join(root, filepath.FromSlash(f.Path))
		case "artifactOutput":
			return artifactOut(outDir, g.Artifacts[f.Artifact])
		}
		return filepath.Join(outDir, f.Path)
	}
	for _, f := range g.Files {
		if f.Kind == "generated" {
			if err := artifact.WriteOutput(filePath(f.ID), []byte(f.Content), 0o644); err != nil {
				return err
			}
		}
	}
	for _, node := range rg.Order {
		var idx int
		fmt.Sscanf(node[1:], "%d", &idx)
		if node[0] == 't' {
			if err := runToolStep(rg, idx, filePath, opts, outDir, cache, stderr); err != nil {
				return err
			}
			continue
		}
		if err := buildArtifact(p, rg, idx, filePath, outDir, opts, stderr); err != nil {
			return err
		}
	}
	return nil
}

// runToolStep runs one registered tool or earlier-built artifact with $in/$out/$secret filled in; a secret is copied to a private file for the run and removed afterward.
// A step whose action key matches a recorded one restores that action's outputs instead of running, so an unchanged tool, argv, environment and input set never reruns.
func runToolStep(rg *ResolvedGraph, idx int, filePath func(int) string, opts ExecOptions, outDir string, cache *actionCache, stderr io.Writer) error {
	secrets := opts.Secrets
	s := rg.Graph.ToolSteps[idx]
	name := s.Tool
	var program string
	var prefix, extraEnv []string
	if s.Kind == "artifact" {
		art := rg.Graph.Artifacts[s.Artifact]
		name, program = art.Name, artifactOut(outDir, art)
	} else {
		tp, _ := rg.Frameworks[s.Framework].program(s.Tool)
		program, prefix, extraEnv = tp.Path, tp.Args, tp.Env
	}
	var secretFiles []string
	defer func() {
		for _, f := range secretFiles {
			wipeAndRemove(f)
		}
	}()
	args := make([]string, 0, len(s.Args))
	display := make([]string, 0, len(s.Args))
	for _, a := range s.Args {
		if a != "" && (strings.Contains(a[1:], "$in:") || strings.Contains(a[1:], "$out:") || strings.Contains(a[1:], "$secret:")) {
			return fmt.Errorf("tool %s: a placeholder must be a whole argument, not part of %q", name, a)
		}
		d := a
		switch {
		case strings.HasPrefix(a, "$in:"):
			i, err := placeholderIndex(a, len(s.Inputs))
			if err != nil {
				return fmt.Errorf("tool %s: %w", name, err)
			}
			a = filePath(s.Inputs[i])
			d = a
		case strings.HasPrefix(a, "$out:"):
			i, err := placeholderIndex(a, len(s.Outputs))
			if err != nil {
				return fmt.Errorf("tool %s: %w", name, err)
			}
			a = filePath(s.Outputs[i])
			d = a
		case strings.HasPrefix(a, "$secret:"):
			i, err := placeholderIndex(a, len(s.Secrets))
			if err != nil {
				return fmt.Errorf("tool %s: %w", name, err)
			}
			secretName := rg.Graph.Secrets[s.Secrets[i]].Name
			src, ok := secrets[secretName]
			if !ok {
				return fmt.Errorf("tool %s needs secret %s; give --secret %s=<file> or KIGUMI_SECRET_%s", name, secretName, secretName, secretName)
			}
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("secret %s: %w", secretName, err)
			}
			tmp, err := os.CreateTemp("", "kigumi-secret-")
			if err != nil {
				zeroBytes(data)
				return err
			}
			if _, err := tmp.Write(data); err != nil {
				zeroBytes(data)
				tmp.Close()
				wipeAndRemove(tmp.Name())
				return fmt.Errorf("secret %s: %w", secretName, err)
			}
			zeroBytes(data)
			if err := tmp.Close(); err != nil {
				wipeAndRemove(tmp.Name())
				return fmt.Errorf("secret %s: %w", secretName, err)
			}
			secretFiles = append(secretFiles, tmp.Name())
			a = tmp.Name()
			d = "$secret:" + secretName
		}
		args = append(args, a)
		display = append(display, d)
	}
	outPaths := make([]string, len(s.Outputs))
	outNames := make([]string, len(s.Outputs))
	for i, o := range s.Outputs {
		p := filePath(o)
		if err := artifact.CheckOutputSlot(p); err != nil {
			return fmt.Errorf("tool %s: %w", name, err)
		}
		outPaths[i] = p
		outNames[i] = rg.Graph.Files[o].Path
	}
	// The child environment is canonicalized once: the action key hashes
	// exactly what the process sees, so a reordered allowlist cannot split it.
	// A bare tool name is resolved through the compiler options' search path,
	// never the driver's own environment, and the resolved path is what both
	// the action key digests and the launch runs.
	program, resolvable := resolveProgram(rg.options, program)
	env := artifact.CanonicalEnv(append(toolEnv(opts.ToolEnv, rg.Graph.Request.Target), extraEnv...))
	fullArgs := append(append([]string{}, prefix...), args...)
	fullDisplay := append(append([]string{}, prefix...), display...)
	// The key hashes the step's arguments with their placeholders intact:
	// resolved paths move with the output directory, while the content they
	// name is covered by the input digests and the declared output names.
	keyArgs := append(append([]string{}, prefix...), s.Args...)
	// A step that takes a secret is never cached: the secret's bytes are not
	// part of the key, and hashing them would record a fingerprint of it.
	cacheable := cache != nil && resolvable && len(s.Secrets) == 0
	var action artifact.Action
	if cacheable {
		action, cacheable = toolAction(cache, rg, s, program, keyArgs, env, filePath, outNames)
	}
	if cacheable {
		key := action.Key()
		if cache.lookup(key, outPaths) {
			cache.stats.Hits++
			fmt.Fprintf(stderr, "build: tool %s reuses cached outputs (action %s)\n", name, artifact.ShortHash(key))
			return nil
		}
		cache.stats.Misses++
	} else if cache != nil {
		cache.stats.Uncacheable++
	}
	quarantined, err := artifact.Quarantine(outPaths)
	if err != nil {
		return fmt.Errorf("tool %s: quarantine a stale output: %w", name, err)
	}
	fmt.Fprintf(stderr, "build: tool %s runs trusted (no sandbox), env %s\n", name, envNames(env))
	var runErr error
	for attempt := 0; attempt < 100; attempt++ {
		cmd := exec.Command(program, fullArgs...)
		cmd.Dir = outDir
		cmd.Env = env
		cmd.Stderr = stderr
		cmd.Stdout = stderr
		runErr = cmd.Run()
		if runErr == nil || !errors.Is(runErr, syscall.ETXTBSY) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if runErr != nil {
		artifact.RestoreQuarantined(outPaths, quarantined)
		return fmt.Errorf("tool %s (%s %s) failed: %w", name, program, strings.Join(fullDisplay, " "), runErr)
	}
	for _, o := range s.Outputs {
		p := filePath(o)
		fi, err := os.Lstat(p)
		if err != nil || !fi.Mode().IsRegular() {
			artifact.RestoreQuarantined(outPaths, quarantined)
			return fmt.Errorf("tool %s did not produce a regular file at %s", name, rg.Graph.Files[o].Path)
		}
	}
	artifact.DropQuarantined(quarantined)
	if cacheable {
		if err := cache.store(action.Key(), outPaths, outNames); err != nil {
			fmt.Fprintf(stderr, "note: tool %s: its action was not recorded for reuse: %v\n", name, err)
		} else {
			cache.stats.Stores++
		}
	}
	return nil
}

package audit

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/native/object"
)

// Archive checks a `--sys none` archive's embedder contract: every undefined
// symbol must be one freestanding_symbols.txt or an Embedder record names, and nothing is thread-local; name is the output path the messages quote.
func Archive(o buildconfig.CompilerOptions, ar []string, archive, name string, allowed map[string]bool, embedder []string) error {
	dir, err := os.MkdirTemp("", "kigumi-audit")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	absArchive, err := o.Abs(archive)
	if err != nil {
		return err
	}
	cmd := exec.Command(ar[0], append(append([]string{}, ar[1:]...), "x", absArchive)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("auditing %s: extracting archive: %w\n%s", name, err, out)
	}
	members, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	defined := map[string]bool{}
	var allSyms []object.Symbol
	for _, mem := range members {
		syms, err := object.ReadSymbols(filepath.Join(dir, mem.Name()))
		if err != nil {
			return fmt.Errorf("auditing %s: %w", name, err)
		}
		for _, s := range syms {
			if s.Defined {
				defined[s.Name] = true
			}
		}
		allSyms = append(allSyms, syms...)
	}
	permitted := make(map[string]bool, len(allowed)+len(embedder))
	maps.Copy(permitted, allowed)
	for _, s := range embedder {
		permitted[s] = true
	}
	var missing []string
	seen := map[string]bool{}
	for _, s := range allSyms {
		if s.TLS {
			return fmt.Errorf("auditing %s: %q is thread-local; freestanding has no TLS runtime", name, s.Name)
		}
		if s.Defined || defined[s.Name] || permitted[s.Name] || seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		missing = append(missing, s.Name)
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	if cat := object.Classify(missing[0]); cat != "" {
		return fmt.Errorf("auditing %s: references undefined symbol %q (%s), which is not in internal/backend/llgen/runtimegen/runtime/freestanding_symbols.txt (all: %s); if the embedder supplies it, name it in an Embedder record of mod.kg", name, missing[0], cat, strings.Join(missing, ", "))
	}
	return fmt.Errorf("auditing %s: references undefined symbol %q, which is not in internal/backend/llgen/runtimegen/runtime/freestanding_symbols.txt (all: %s); if the embedder supplies it, name it in an Embedder record of mod.kg", name, missing[0], strings.Join(missing, ", "))
}

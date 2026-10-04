package driver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"kigumi/internal/llgen"
)

// auditBareArchive checks a `--sys none` archive's embedder contract: every undefined
// symbol must be one freestanding_symbols.txt names, and nothing is thread-local.
func auditBareArchive(cc []string, archive string) error {
	dir, err := os.MkdirTemp("", "kigumi-audit")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	ar := arTool(cc)
	absArchive, err := filepath.Abs(archive)
	if err != nil {
		return err
	}
	cmd := exec.Command(ar[0], append(append([]string{}, ar[1:]...), "x", absArchive)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("auditing %s: extracting archive: %w\n%s", archive, err, out)
	}
	members, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	defined := map[string]bool{}
	var allSyms []objSymbol
	for _, mem := range members {
		syms, err := readObjectSymbols(filepath.Join(dir, mem.Name()))
		if err != nil {
			return fmt.Errorf("auditing %s: %w", archive, err)
		}
		for _, s := range syms {
			if s.Defined {
				defined[s.Name] = true
			}
		}
		allSyms = append(allSyms, syms...)
	}
	allowed := llgen.FreestandingSymbols()
	for _, s := range allSyms {
		if s.TLS {
			return fmt.Errorf("auditing %s: %q is thread-local; freestanding has no TLS runtime", archive, s.Name)
		}
		if s.Defined || defined[s.Name] {
			continue
		}
		if !allowed[s.Name] {
			if cat := classifySymbol(s.Name); cat != "" {
				return fmt.Errorf("auditing %s: references undefined symbol %q (%s), which is not in internal/llgen/runtime/freestanding_symbols.txt", archive, s.Name, cat)
			}
			return fmt.Errorf("auditing %s: references undefined symbol %q, which is not in internal/llgen/runtime/freestanding_symbols.txt", archive, s.Name)
		}
	}
	return nil
}

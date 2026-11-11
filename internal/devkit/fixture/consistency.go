package fixture

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// GroupFromPath returns the single directory under testdata/<kind> that holds
// path, or "" when the fixture sits directly in the kind directory.
func GroupFromPath(p string) string {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(p)), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "testdata" {
			continue
		}
		if i+3 >= len(parts) {
			return ""
		}
		return parts[i+2]
	}
	return ""
}

// Consistency reports every manifest/disk/catalog mismatch, one error each.
func Consistency(repoRoot string, m *Manifest) []error {
	var errs []error

	disk, err := ScanDisk(repoRoot)
	if err != nil {
		return []error{err}
	}
	byID := map[string]int{}
	for i, fx := range m.Fixtures {
		if prev, dup := byID[fx.ID]; dup {
			errs = append(errs, fmt.Errorf("duplicate fixture id %q (entries %d and %d)", fx.ID, prev, i))
			continue
		}
		byID[fx.ID] = i
	}

	for id, rel := range disk {
		if _, ok := byID[id]; !ok {
			errs = append(errs, fmt.Errorf("%s: fixture on disk has no manifest entry (id %q)", rel, id))
		}
	}

	basenames := map[string]string{} // kind/name -> path, to catch two groups claiming one id
	for _, fx := range m.Fixtures {
		kind, base, ok := strings.Cut(fx.ID, "/")
		if !ok || kind != fx.Kind || base == "" {
			errs = append(errs, fmt.Errorf("manifest entry %q: id and kind %q do not agree", fx.ID, fx.Kind))
			continue
		}
		if prev, dup := basenames[fx.ID]; dup {
			errs = append(errs, fmt.Errorf("manifest entry %q: basename collides with %s", fx.ID, prev))
		}
		basenames[fx.ID] = fx.Path

		ext := Ext(fx.Kind)
		prefix := path.Join("testdata", fx.Kind) + "/"
		if !strings.HasPrefix(fx.Path, prefix) || filepath.Ext(fx.Path) != ext {
			errs = append(errs, fmt.Errorf("manifest entry %q: path %q must live under %s and end in %s", fx.ID, fx.Path, prefix, ext))
			continue
		}
		if got := strings.TrimSuffix(path.Base(fx.Path), ext); got != base {
			errs = append(errs, fmt.Errorf("manifest entry %q: path %q names %q", fx.ID, fx.Path, got))
		}
		rel := strings.TrimPrefix(fx.Path, prefix)
		depth := strings.Count(rel, "/")
		if depth > 1 {
			errs = append(errs, fmt.Errorf("manifest entry %q: path %q nests deeper than kind/group", fx.ID, fx.Path))
		}
		group := GroupFromPath(fx.Path)
		if fx.Group != group {
			errs = append(errs, fmt.Errorf("manifest entry %q: group %q does not match path %q (want %q)", fx.ID, fx.Group, fx.Path, group))
		}
		if group == "misc" {
			errs = append(errs, fmt.Errorf("manifest entry %q: group %q is not a responsibility; assign the nearest real group", fx.ID, group))
		}
		if fx.Stage == "" || !slices.Contains(StageUniverse[fx.Kind], fx.Stage) {
			errs = append(errs, fmt.Errorf("manifest entry %q: stage %q is not a stage of kind %q", fx.ID, fx.Stage, fx.Kind))
		}

		modes, known := EngineUniverse[fx.Kind]
		if !known {
			errs = append(errs, fmt.Errorf("manifest entry %q: unknown fixture kind %q", fx.ID, fx.Kind))
		} else {
			for _, mode := range fx.Mode {
				if !slices.Contains(modes, mode) {
					errs = append(errs, fmt.Errorf("manifest entry %q: mode %q is not an engine of kind %q", fx.ID, mode, fx.Kind))
				}
			}
		}
		if _, onDisk := disk[fx.ID]; !onDisk {
			errs = append(errs, fmt.Errorf("manifest entry %q: no such fixture on disk (expected %s)", fx.ID, fx.Path))
			continue
		}
		if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(fx.Path))); err != nil {
			errs = append(errs, fmt.Errorf("manifest entry %q: path %s does not exist", fx.ID, fx.Path))
		}
		universe := EngineUniverse[fx.Kind]
		for _, engine := range universe {
			if fx.Runs(engine) {
				continue
			}
			reason, excluded := fx.Skip[engine]
			if !excluded {
				errs = append(errs, fmt.Errorf("%s: %s is excluded from mode but has no skip reason", fx.ID, engine))
				continue
			}
			if _, known := m.Reasons[reason]; !known {
				errs = append(errs, fmt.Errorf("%s: skip reason %q for %s is not in the reason catalog", fx.ID, reason, engine))
			}
		}

		if fx.Kind != "oom" {
			if fx.Pair != "" || len(fx.Covers) > 0 {
				errs = append(errs, fmt.Errorf("manifest entry %q: pair/covers belong to oom fixtures only", fx.ID))
			}
			continue
		}
		if len(fx.Covers) > 0 && fx.Pair == "" {
			errs = append(errs, fmt.Errorf("manifest entry %q: covers %v but has no pair fixture", fx.ID, fx.Covers))
		}
		if fx.Pair == "" {
			continue
		}
		pair := m.ByID(fx.Pair)
		switch {
		case pair == nil:
			errs = append(errs, fmt.Errorf("manifest entry %q: pair %q has no manifest entry", fx.ID, fx.Pair))
		case pair.Kind != "oom":
			errs = append(errs, fmt.Errorf("manifest entry %q: pair %q has kind %q, want oom", fx.ID, fx.Pair, pair.Kind))
		case pair.Group != fx.Group:
			errs = append(errs, fmt.Errorf("manifest entry %q: pair %q is in group %q, want %q", fx.ID, fx.Pair, pair.Group, fx.Group))
		case pair.Pair != fx.ID:
			errs = append(errs, fmt.Errorf("manifest entry %q: pair %q points back to %q", fx.ID, fx.Pair, pair.Pair))
		}
	}
	return errs
}

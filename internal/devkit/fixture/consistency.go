package fixture

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

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

	for _, fx := range m.Fixtures {
		base := strings.TrimSuffix(path.Base(fx.Path), ".txtar")
		if fx.ID != fx.Kind+"/"+base || fx.Path != "testdata/"+fx.Kind+"/"+base+".txtar" {
			errs = append(errs, fmt.Errorf("manifest entry %q: path %q and id %q do not agree on kind and name", fx.ID, fx.Path, fx.ID))
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
	}
	return errs
}

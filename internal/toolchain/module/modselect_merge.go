package module

import "fmt"

func mergeRequire(max map[string]Require, by map[string]string, r Require, from string) error {
	cur, ok := max[r.Name]
	switch {
	case !ok:
		max[r.Name], by[r.Name] = r, from
	case r.Version == cur.Version:
		if cur.URL == "" {
			cur.URL = r.URL
			max[r.Name] = cur
		}
	case CompareVersions(r.Version, cur.Version) == 0:
		// CacheDir keys off the literal tag, so build-metadata variants are not necessarily the same content.
		return fmt.Errorf("dependency %s is required at %s by %s and at %s by %s: same precedence but different tags", r.Name, cur.Version, by[r.Name], r.Version, from)
	default:
		newer, err := versionNewer(r, cur)
		if err != nil {
			return fmt.Errorf("dependency %s is required at %s by %s and at %s by %s: %w", r.Name, cur.Version, by[r.Name], r.Version, from, err)
		}
		if newer {
			if r.URL == "" {
				r.URL = cur.URL
			}
			max[r.Name], by[r.Name] = r, from
		}
	}
	return nil
}

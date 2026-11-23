package module

import "fmt"

func mergeRequire(max map[string]Require, by map[string]string, r Require, from string) error {
	key := r.Key()
	cur, ok := max[key]
	switch {
	case !ok:
		max[key], by[key] = r, from
	case r.Name != cur.Name:
		return fmt.Errorf("dependency key %s names both %s and %s; aliases must be unique across the graph", key, cur.Name, r.Name)
	case r.Version == cur.Version:
		if cur.URL == "" {
			cur.URL = r.URL
			max[key] = cur
		}
	case CompareVersions(r.Version, cur.Version) == 0:
		// CacheDir keys off the literal tag, so build-metadata variants are not necessarily the same content.
		return fmt.Errorf("dependency %s is required at %s by %s and at %s by %s: same precedence but different tags", DescribeRequire(r), cur.Version, by[key], r.Version, from)
	default:
		newer, err := versionNewer(r, cur)
		if err != nil {
			return fmt.Errorf("dependency %s is required at %s by %s and at %s by %s: %w", DescribeRequire(r), cur.Version, by[key], r.Version, from, err)
		}
		if newer {
			if r.URL == "" {
				r.URL = cur.URL
			}
			max[key], by[key] = r, from
		}
	}
	return nil
}

// DescribeRequire names a dependency in a diagnostic, showing the alias when
// it differs from the real module name.
func DescribeRequire(r Require) string {
	if r.Alias != "" {
		return r.Name + " (alias " + r.Alias + ")"
	}
	return r.Name
}

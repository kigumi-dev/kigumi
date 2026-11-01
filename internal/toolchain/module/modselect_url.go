package module

import (
	"fmt"
	"strings"
)

func RepoURLKey(r Require) string {
	url, _, ok := RepoOf(r)
	if !ok {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
}

type urlClaim struct{ url, by string }

type moduleClaims struct{ root, explicit, derived *urlClaim }

type urlClaims map[string]*moduleClaims

// A url derived from the name conflicts with an explicit transitive url only when the root pinned nothing.
func (c urlClaims) claim(r Require, from string, isRoot bool) error {
	key := RepoURLKey(r)
	if key == "" {
		return nil
	}
	m := c[r.Name]
	if m == nil {
		m = &moduleClaims{}
		c[r.Name] = m
	}
	here := &urlClaim{key, from}
	switch {
	case isRoot:
		if m.root == nil {
			m.root = here
		} else if m.root.url != key {
			return urlMoved(r.Name, m.root, here)
		}
	case r.URL != "":
		if m.explicit == nil {
			m.explicit = here
		}
	default:
		if m.derived == nil {
			m.derived = here
		}
	}
	against := m.root
	if against == nil {
		against = m.explicit
	}
	if against != nil && r.URL != "" && !isRoot && against.url != key {
		return urlMoved(r.Name, against, here)
	}
	if m.root == nil && m.explicit != nil && m.derived != nil && m.explicit.url != m.derived.url {
		return urlMoved(r.Name, m.derived, m.explicit)
	}
	return nil
}

func urlMoved(name string, was, now *urlClaim) error {
	return fmt.Errorf("dependency %s is fetched from %s per %s but %s requires it from %s; a dependency cannot move where a module comes from", name, was.url, was.by, now.by, now.url)
}

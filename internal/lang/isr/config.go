package isr

import (
	"fmt"
	"slices"
	"strings"
)

// ConfigVersion moves when the meaning of Config or of a contract identity changes.
const ConfigVersion = 2

// Config is the root module's resolved interrupt configuration: Root is both the root
// limit and the common configuration; Handlers narrows it for the handler keyed "pkg/path.name".
type Config struct {
	Root     []string
	Handlers map[string][]string
}

func (c Config) Canonical() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "v%d root=%s", ConfigVersion, strings.Join(c.Root, ";"))
	keys := make([]string, 0, len(c.Handlers))
	for k := range c.Handlers {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(&sb, " %s=%s", k, strings.Join(c.Handlers[k], ";"))
	}
	return sb.String()
}

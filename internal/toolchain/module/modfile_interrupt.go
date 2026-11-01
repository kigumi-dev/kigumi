package module

import (
	"slices"
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

// addInterrupt reads one provisional `Interrupt { handler, allow }` record: without `handler` it is the
// root limit and common configuration; with one it narrows that for each comma-separated handler key.
func (mf *ModFile) addInterrupt(r Record) []diag.Diagnostic {
	errf := func(msg string) []diag.Diagnostic {
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, msg)}
	}
	var allow []string
	for _, text := range strings.Split(r.Fields["allow"], ";") {
		if strings.TrimSpace(text) == "" {
			continue
		}
		c, err := sem.ParseIsrPermission(text)
		if err != nil {
			return errf("Interrupt allow: " + err.Error())
		}
		if c.Kind == sem.IsrInert {
			return errf("Interrupt allow names MMIO or synchronization operations; ordinary computation needs no permission")
		}
		allow = append(allow, c.ID)
	}
	slices.Sort(allow)
	allow = slices.Compact(allow)
	cfg := &mf.Interrupt
	if strings.TrimSpace(r.Fields["handler"]) == "" {
		if cfg.Root != nil {
			return errf("only one Interrupt record may omit `handler`: it is the root limit")
		}
		cfg.Root = append([]string{}, allow...)
		return nil
	}
	if cfg.Handlers == nil {
		cfg.Handlers = map[string][]string{}
	}
	for _, key := range strings.Split(r.Fields["handler"], ",") {
		key = strings.TrimSpace(key)
		if i := strings.LastIndexByte(key, '.'); i <= 0 || i == len(key)-1 {
			return errf("Interrupt handler `" + key + "` must be written `pkg/path.name`")
		}
		if _, dup := cfg.Handlers[key]; dup {
			return errf("Interrupt handler `" + key + "` is configured twice")
		}
		cfg.Handlers[key] = allow
	}
	return nil
}

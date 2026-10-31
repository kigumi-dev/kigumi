package llgen

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// siteText is the `file:line` of in, as a C string global; the VM reports the same text.
func (e *emitter) siteText(f *midir.Func, in midir.Inst) string {
	return e.cstr(e.p.SiteAt(f, in.Node).Text())
}

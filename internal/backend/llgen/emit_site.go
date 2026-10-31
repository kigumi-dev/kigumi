package llgen

import "kigumi/internal/compiler/mir"

// siteText is the `file:line` of in, as a C string global; the VM reports the same text.
func (e *emitter) siteText(f *mir.Func, in mir.Inst) string {
	return e.cstr(e.p.SiteAt(f, in.Node).Text())
}

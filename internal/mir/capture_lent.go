package mir

import "kigumi/internal/sem"

// CaptureLent reports that the body never drops its i-th parameter.
func (p *Program) CaptureLent(f *Func, i int) bool {
	l := f.Locals[f.Params[i]]
	return l.Borrowed || p.R.Types.Kind(l.Type) == sem.KRef
}

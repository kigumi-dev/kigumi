package mir

import "kigumi/internal/lang/diag"

// HasErrors reports whether any function carries an ownership error.
func (p *Program) HasErrors() bool {
	for _, f := range p.Funcs {
		if len(f.Diags) > 0 {
			return true
		}
	}
	return false
}

// Render prints the ownership diagnostics of every function.
func (p *Program) Render() string {
	out := ""
	for _, f := range p.Funcs {
		if len(f.Diags) == 0 {
			continue
		}
		out += diag.RenderAll(p.R.Tree(f.File).File, f.Diags)
	}
	return out
}

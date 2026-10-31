package borrow

func (e *erasedReaders) lentReported(w erasedWrite) bool {
	t := e.p.R.Tree(e.f.File)
	write := t.Span(w.write)
	for key := range e.prior {
		if key.code != "borrow-conflict" || key.local != w.root {
			continue
		}
		if key.node == w.write {
			return true
		}
		prior := t.Span(key.node)
		if write.Start <= prior.Start && prior.End <= write.End {
			return true
		}
	}
	return false
}

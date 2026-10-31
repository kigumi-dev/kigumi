package ir

func (f *Func) IsDirect(l LocalID) bool { return int(l) < len(f.Direct) && f.Direct[l] }

func (f *Func) DirectPlanned() bool { return f.Direct != nil }

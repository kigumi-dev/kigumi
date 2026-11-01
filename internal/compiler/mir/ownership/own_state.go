package ownership

// Ownership states of a move-only local along a path.
type ownState uint8

const (
	ownUninit ownState = iota
	ownValid
	ownMoved
	ownConflict
	// ownMaybe joins assigned and never-assigned paths: a pattern binding
	// behind `&&`, which the checker only lets the matched path read.
	ownMaybe
)

// joinOwn merges two states at a block entry: a move against a value is a
// conflict, an unassigned side only makes the value conditional.
func joinOwn(a, b ownState) ownState {
	if a == b {
		return a
	}
	if a == ownUninit && b == ownMoved || a == ownMoved && b == ownUninit {
		return ownMoved
	}
	if a == ownMoved || b == ownMoved || a == ownConflict || b == ownConflict {
		return ownConflict
	}
	return ownMaybe
}

// val and field are independent: a moved-out field's temporary stays valid in val until consumed.
// called is whether a once-only callable has run; callAt is where, apart from at so a move cannot hide the call.
type dstate struct {
	val, field, called, rel ownVec
	at, callAt              nodeVec
}

func newDstate() *dstate { return &dstate{} }

func cloneD(s *dstate) *dstate {
	return &dstate{val: s.val.clone(), field: s.field.clone(), called: s.called.clone(), rel: s.rel.clone(), at: s.at.clone(), callAt: s.callAt.clone()}
}

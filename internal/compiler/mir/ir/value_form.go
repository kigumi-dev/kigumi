package ir

// ValueForm is what an instruction builds as a callable: no OpClosure, a closure literal, a method value holding its
// receiver, a requirement's method value picked by a witness, a method value applying a bound fn to its receiver, or an
// OpClosure whose tag names none of those, which every consumer must refuse.
type ValueForm uint8

const (
	FormNone ValueForm = iota
	FormClosure
	FormMethod
	FormWitness
	FormApplied
	FormUnknown
)

var formTags = map[ValueForm]string{FormClosure: "", FormMethod: "method", FormWitness: "witness", FormApplied: "applied"}

// Tag is the Str an OpClosure of form f carries; FormNone and FormUnknown have none.
func (f ValueForm) Tag() string {
	tag, ok := formTags[f]
	if !ok {
		panic("ir: no OpClosure tag for this ValueForm")
	}
	return tag
}

// Form is the form in builds.
func (in *Inst) Form() ValueForm {
	if in.Op != OpClosure {
		return FormNone
	}
	for f, tag := range formTags {
		if tag == in.Str {
			return f
		}
	}
	return FormUnknown
}

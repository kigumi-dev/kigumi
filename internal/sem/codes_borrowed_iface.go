package sem

var (
	cAsyncWitness        = defineCode("E2191", "async-witness-mismatch", "`{name}` is {got}, the requirement is {want}; a requirement and its method must agree on `async`")
	cBorrowInExistential = defineCode("E2192", "borrow-in-existential", "a value carrying a borrow cannot become `{type}`: an interface value does not carry the borrow's lifetime; keep the concrete type, or a generic `T` constrained by the interface, while the borrow is live, or convert a value it can own")
	cLentViewReused      = defineCode("E2193", "lent-view-reused", "this `&Interface` view is made again while the one it made on an earlier pass is still in use; keep each view inside the loop body that makes it")
)

func (r *Result) asyncWitnessWhy(m EntityID, async bool) string {
	got, want := "`async`", "synchronous"
	if !async {
		got, want = want, got
	}
	return r.noteText(cAsyncWitness.Template, r.Entities[m].Name, got, want)
}

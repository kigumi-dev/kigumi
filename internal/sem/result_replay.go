package sem

// replayedCheck stands in for a check whose outcome was settled earlier and replayed from the package cache: it carries the rendered diagnostics and nothing else.
type replayedCheck struct {
	text      string
	hasErrors bool
}

// ReplayResult is a Result with no program behind it, answering only Render and HasErrors.
func ReplayResult(text string, hasErrors bool) *Result {
	return &Result{replay: &replayedCheck{text: text, hasErrors: hasErrors}}
}

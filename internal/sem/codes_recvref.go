package sem

// Diagnostic catalog, E153x: a `mut self` receiver reached through a reference naming no place of the caller's.
var (
	cRecvRefNoPlace = defineCode("E1530", "recv-ref-no-place", "{what} is the receiver here, and `{name}` replaces its receiver as a whole with no place of this body's to put the new value; call it on the place that owns the value, or on a field or element of one")
)

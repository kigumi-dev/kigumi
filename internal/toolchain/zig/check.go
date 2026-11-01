package zig

// Check is one resolution attempt: the zig Resolve found (if any), the
// version it reports when Want is pinned, and whether that matched.
type Check struct {
	// Path is "" when no usable zig was found at all.
	Path string
	// Version is "" unless Want is pinned and the resolved zig answered
	// `zig version`.
	Version string
	// Want is the pinned version Check was asked to verify; "" means unpinned.
	Want string
	// Mismatch is set when Want is pinned and Version differs from it.
	Mismatch bool
	// Err is the resolution or `zig version` failure; set only when Want is
	// pinned, or an explicit override was given and failed to resolve.
	Err error
}

// CheckZig resolves zig (explicit, then PATH) and, when want is pinned,
// verifies the resolved version matches it exactly; want is "" when unpinned.
func CheckZig(look Lookup, explicit, want string) Check {
	path, err := Resolve(look, explicit)
	if path == "" {
		if explicit == "" && want == "" {
			return Check{}
		}
		return Check{Want: want, Err: err}
	}
	c := Check{Path: path, Want: want}
	if want == "" {
		return c
	}
	v, err := Version(path)
	if err != nil {
		c.Err = err
		return c
	}
	c.Version = v
	c.Mismatch = v != want
	return c
}

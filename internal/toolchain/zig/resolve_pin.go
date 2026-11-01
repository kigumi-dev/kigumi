package zig

import "kigumi/internal/toolchain/zig/store"

// Resolved is one module's fully-resolved zig: the path to use, where it
// came from, and (when the module is pinned) whether it satisfies the pin.
type Resolved struct {
	Path   string
	Source string // "managed store", "$KIGUMI_ZIG", or "PATH"
	Check
}

// ForModule resolves zig in kigumi's own order: the managed store's exact
// pin, then explicit (KIGUMI_ZIG), then PATH; "" storeRoot or want skips the store.
func ForModule(look Lookup, storeRoot, explicit, want string) Resolved {
	if want != "" && storeRoot != "" {
		if path, ok := store.Installed(storeRoot, want); ok {
			return Resolved{Path: path, Source: "managed store", Check: Check{Path: path, Version: want, Want: want}}
		}
	}
	source := "PATH"
	if explicit != "" {
		source = "$KIGUMI_ZIG"
	}
	c := CheckZig(look, explicit, want)
	return Resolved{Path: c.Path, Source: source, Check: c}
}

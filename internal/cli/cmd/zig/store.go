package zig

import (
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
)

// storeRoot resolves the managed store's directory: --zig-home, else session.ZigStoreRoot's default.
func storeRoot(zigHome string) (string, error) {
	if zigHome != "" {
		return session.CompilerOptions().ZigStoreRoot(zigHome)
	}
	return session.ZigStoreRoot()
}

// modulePin is the zig version the module in the working directory pins, or "" when it has none.
func modulePin() string {
	mf, _, err := driver.ReadModFile(".")
	if err != nil {
		return ""
	}
	return mf.Zig
}

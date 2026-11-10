//go:build !unix

package driver

// ownedBySelf has no portable owner check outside unix; the 0700 mode is all that can be enforced there.
func ownedBySelf(string) bool { return true }

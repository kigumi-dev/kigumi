package mir

import "fmt"

// ProvisionKey names the part of p that Provides reads. The POSIX hosted OSes share one key because no rule or reason text tells them apart.
func (p Platform) ProvisionKey() string {
	os := p.OS
	if !p.Bare && !p.NoLibc && PosixSignalOS(os) {
		os = "posix"
	}
	return fmt.Sprintf("%s|%t|%t|%q|%t|%q", os, p.Bare, p.NoLibc, p.Allocator, p.RecoverableOnly, p.ThreadProfile)
}

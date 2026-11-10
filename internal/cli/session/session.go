// Package session resolves what a command runs against: the std root, target and policy from flags and environment, and the module they load.
package session

import (
	"github.com/spf13/cobra"

	"kigumi/internal/toolchain/module"
)

// Session is the flag set one run resolves against, read once so that run code never sees cobra.
type Session struct {
	Std              string
	NoLocal          bool
	Offline          bool
	Release          bool
	Triple           string
	Sys              string
	Allocator        string
	Profile          string
	BlockingExecutor string
	StackProfile     string
	StackBudgetKiB   int
}

// Read takes the session flags from cmd; a flag cmd does not define reads as its zero value.
func Read(cmd *cobra.Command) Session {
	f := cmd.Flags()
	var s Session
	s.Std, _ = f.GetString("std")
	s.NoLocal, _ = f.GetBool("no-local")
	s.Offline, _ = f.GetBool("offline")
	s.Release, _ = f.GetBool("release")
	s.Triple, _ = f.GetString("target")
	s.Sys, _ = f.GetString("sys")
	s.Allocator, _ = f.GetString("allocator")
	s.Profile, _ = f.GetString("profile")
	s.BlockingExecutor, _ = f.GetString("blocking-executor")
	s.StackProfile, _ = f.GetString("stack-profile")
	s.StackBudgetKiB, _ = f.GetInt("stack-budget-kib")
	return s
}

// ReadLocal is the mod.local.kg overrides a command sees at root: none under --no-local or KIGUMI_LOCAL=off, the same switch the loader honors.
func (s Session) ReadLocal(root string) (module.Local, error) {
	if s.NoLocal || LocalOff() {
		return module.Local{Replaces: map[string]string{}, Provenance: map[string]string{}}, nil
	}
	return module.ReadLocal(CompilerOptions(), root)
}

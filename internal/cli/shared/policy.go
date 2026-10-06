package shared

import (
	"os"

	"github.com/spf13/cobra"

	"kigumi/internal/driver"
)

// ReleaseFlag adds --release to cmd; Policy reads it back.
func ReleaseFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("release", false, "require a locked, hash-verified dependency tree and reject mod.local.kg and ambient environment overrides")
}

// Policy is the only place a bool decides between the two build policies;
// everything downstream reads BuildPolicy's fields instead.
func Policy(cmd *cobra.Command) driver.BuildPolicy {
	if release, _ := cmd.Flags().GetBool("release"); release {
		return driver.Release()
	}
	return driver.Development()
}

// Offline reports --offline: stop a pinned zig build from fetching the pin
// on demand, reporting a diagnostic instead.
func Offline(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("offline")
	return v
}

// AmbientStd reports $KIGUMI_STD when --std did not pin the std root
// instead; env reads stay in this package, never in internal/driver.
func AmbientStd(cmd *cobra.Command) (path string, ambient bool) {
	if std, _ := cmd.Flags().GetString("std"); std != "" {
		return "", false
	}
	if env := os.Getenv("KIGUMI_STD"); env != "" {
		return env, true
	}
	return "", false
}

// AmbientCFlags reports KIGUMI_CFLAGS: there is no --cflags flag, so any
// extra compiler/linker flag in effect today is always ambient.
func AmbientCFlags() (flags []string, ambient bool) {
	f := CFlags()
	return f, len(f) > 0
}

// ReleaseViolations collects the ambient-environment violations only the
// CLI can see, for EvaluateRelease's extra argument.
func ReleaseViolations(cmd *cobra.Command, includeCFlags bool) []driver.Violation {
	var vs []driver.Violation
	if path, ambient := AmbientStd(cmd); ambient {
		vs = append(vs, driver.AmbientStdViolation(path))
	}
	if includeCFlags {
		if flags, ambient := AmbientCFlags(); ambient {
			vs = append(vs, driver.AmbientCFlagsViolation(flags))
		}
	}
	return vs
}

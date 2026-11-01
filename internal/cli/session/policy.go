package session

import (
	"os"

	"github.com/spf13/cobra"

	"kigumi/internal/toolchain/policy"
)

// ReleaseFlag adds --release to cmd; Session.Policy reads it back.
func ReleaseFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("release", false, "require a locked, hash-verified dependency tree and reject mod.local.kg and ambient environment overrides")
}

// Policy is the only place a bool decides between the two build policies; everything downstream reads BuildPolicy's fields.
func (s Session) Policy() policy.BuildPolicy {
	if s.Release {
		return policy.Release()
	}
	return policy.Development()
}

// AmbientStd reports $KIGUMI_STD when --std did not pin the std root.
func (s Session) AmbientStd() (path string, ambient bool) {
	if s.Std != "" {
		return "", false
	}
	if env := os.Getenv("KIGUMI_STD"); env != "" {
		return env, true
	}
	return "", false
}

// AmbientCFlags reports KIGUMI_CFLAGS; with no --cflags flag, any extra compiler or linker flag is ambient.
func AmbientCFlags() (flags []string, ambient bool) {
	f := CFlags()
	return f, len(f) > 0
}

// ReleaseViolations collects the ambient-environment violations only the CLI can see, for policy.Evaluate's extra argument.
func (s Session) ReleaseViolations(includeCFlags bool) []policy.Violation {
	var vs []policy.Violation
	if path, ambient := s.AmbientStd(); ambient {
		vs = append(vs, policy.AmbientStdViolation(path))
	}
	if includeCFlags {
		if flags, ambient := AmbientCFlags(); ambient {
			vs = append(vs, policy.AmbientCFlagsViolation(flags))
		}
	}
	return vs
}

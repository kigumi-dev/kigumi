package policy

import (
	"fmt"
	"strings"
)

func LocalReplaceViolation(name, path, provenance string) Violation {
	return Violation{
		ID:      "release-local-replace",
		Subject: fmt.Sprintf("%s is resolved from a local directory", name),
		Detail:  []KV{Detail("selected by", provenance), Detail("replacement", path)},
		Policy:  "rejected because the content is not the locked module tree",
		Help:    "update and lock the dependency, or keep this as a development build",
	}
}

func UnlockedScriptViolation(scriptFile, lockPath string) Violation {
	return Violation{
		ID:      "release-unlocked-script",
		Subject: fmt.Sprintf("%s requires dependencies but has no sidecar lock", scriptFile),
		Detail:  []KV{Detail("expected lock", lockPath)},
		Policy:  "rejected because a release build requires every dependency locked with a verified hash",
		Help:    "run `kigumi get --lock` to write " + lockPath + ", or keep this as a development build",
	}
}

func NativeLinkUnresolvedViolation(module, search string) Violation {
	return Violation{
		ID:      "release-native-link-unresolved",
		Subject: fmt.Sprintf("%s declares a Link search path that does not exist", module),
		Detail:  []KV{Detail("search", search)},
		Policy:  "rejected because a release build's native declarations must resolve against the locked tree",
		Help:    "fix the Link search path and re-run `kigumi get`, or keep this as a development build",
	}
}

// NativeLinkIdentityUnknownViolation is distinct from release-native-link-unresolved:
// that one is a missing Search directory, this one a Search directory present but not holding the named library.
func NativeLinkIdentityUnknownViolation(module, library string) Violation {
	return Violation{
		ID:      "release-native-link-identity-unknown",
		Subject: "Link { library: \"" + library + "\" } (" + module + ") does not resolve to a library file this host's search list can find",
		Detail:  []KV{Detail("library", library)},
		Policy:  "rejected because release requires the actual resolved library identity to be recorded, not only the declared name",
		Help:    "add a Link search path that holds lib" + library + ", install the library where this host's linker would find it, or keep this as a development build",
	}
}

// AmbientStdViolation reports that the std library came from $KIGUMI_STD; only
// internal/cli reads the environment, so it builds this violation itself.
func AmbientStdViolation(path string) Violation {
	return Violation{
		ID:      "release-ambient-std",
		Subject: "the std library is selected by $KIGUMI_STD rather than a recorded path",
		Detail:  []KV{Detail("KIGUMI_STD", path)},
		Policy:  "rejected because release requires the toolchain identity to be recorded, not read from the environment",
		Help:    "pass --std explicitly, or keep this as a development build",
	}
}

// AmbientCFlagsViolation reports that extra compiler/linker flags came
// from $KIGUMI_CFLAGS.
func AmbientCFlagsViolation(flags []string) Violation {
	return Violation{
		ID:      "release-ambient-cflags",
		Subject: "the build adds compiler/linker flags from $KIGUMI_CFLAGS",
		Detail:  []KV{Detail("KIGUMI_CFLAGS", strings.Join(flags, " "))},
		Policy:  "rejected because release requires build inputs to be recorded, not read from the environment",
		Help:    "record these flags in the build configuration, or keep this as a development build",
	}
}

// UnverifiedTargetViolation reports a triple whose ABI segment ParseTarget
// could not check against a known list.
func UnverifiedTargetViolation(triple string) Violation {
	return Violation{
		ID:      "release-unverified-target",
		Subject: fmt.Sprintf("target %q has an ABI this build does not recognize", triple),
		Detail:  []KV{Detail("triple", triple)},
		Policy:  "rejected because release requires a target this toolchain has verified support for",
		Help:    "use a recognized triple, or keep this as a development build",
	}
}

// Error renders v as a single build-ending error, for violations that
// must stop the build before release reads unverified content from disk.
func Error(v Violation) error {
	var sb strings.Builder
	renderViolation(&sb, "error", v)
	return fmt.Errorf("%s", strings.TrimRight(sb.String(), "\n"))
}

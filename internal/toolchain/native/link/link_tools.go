package link

import (
	"path/filepath"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/zig"
)

func IsZig(cc []string) bool {
	return len(cc) > 0 && (strings.HasSuffix(cc[0], "zig") || filepath.Base(cc[0]) == "zig")
}

// Archiver is the archiver for cc: zig's own, since a plain system `ar`
// cannot read object files zig cc produced for some targets.
func Archiver(o buildconfig.CompilerOptions, cc []string) ([]string, error) {
	if IsZig(cc) {
		return []string{cc[0], "ar"}, nil
	}
	ar, err := o.LookTool("ar")
	if err != nil {
		return nil, err
	}
	return []string{ar}, nil
}

// ZigVersionOf is "" unless cc is a zig invocation, for the artifact fingerprint
// and the cheader cache key's zig axis.
func ZigVersionOf(cc []string) string {
	if len(cc) == 0 || filepath.Base(cc[0]) != "zig" {
		return ""
	}
	v, err := zigtool.Version(cc[0])
	if err != nil {
		return ""
	}
	return v
}

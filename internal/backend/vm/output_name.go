package vm

import (
	"slices"
	"strings"

	"kigumi/internal/toolchain/buildgraph"
)

// bareOutputName rejects separators, since they could escape the output dir via filepath.Join, and checks Windows-reserved names on every host for cross-platform reproducibility.
func bareOutputName(name string) bool {
	switch {
	case name == "" || name == "." || name == "..":
		return false
	case strings.ContainsAny(name, `/\`):
		return false
	case strings.ContainsAny(name, `<>:"|?*`):
		return false
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return false
	case isWindowsReservedName(name):
		return false
	}
	for _, r := range name {
		if r < 0x20 {
			return false
		}
	}
	return true
}

// outputArtifactName is the bare file name buildArtifact finally links, same as artifactOut computes once outDir is known.
func outputArtifactName(a *buildgraph.Artifact) string {
	if a.Kind == "library" {
		return a.Name + ".a"
	}
	return a.Name
}

// claimOutputName case-folds name for case-insensitive filesystems; only a generated or tool-produced file owns a slot in the output directory, so a literal input and an artifactOutput (whose Path mirrors the Artifact) never collide.
func claimOutputName(g *buildgraph.Graph, name string) (string, bool) {
	key := strings.ToLower(name)
	for _, f := range g.Files {
		if (f.Kind == "generated" || f.Kind == "toolOutput") && strings.ToLower(f.Path) == key {
			return f.Path, true
		}
	}
	for _, a := range g.Artifacts {
		if strings.ToLower(outputArtifactName(a)) == key {
			return "artifact " + a.Name, true
		}
	}
	return "", false
}

// windowsReservedNames are the MS-DOS/Win32 device names CreateFile reserves, with or without an extension.
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM0": true, "COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT0": true, "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

func isWindowsReservedName(name string) bool {
	base, _, _ := strings.Cut(name, ".")
	return windowsReservedNames[strings.ToUpper(base)]
}

// hasReservedPathComponent checks every "/"-segment, since Windows reserves these names at any path component, not just the final one.
func hasReservedPathComponent(dir string) bool {
	for _, seg := range strings.Split(dir, "/") {
		if isWindowsReservedName(seg) {
			return true
		}
	}
	return false
}

func hasParentSegment(dir string) bool {
	return slices.Contains(strings.Split(dir, "/"), "..")
}

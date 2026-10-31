package driver

import (
	"path/filepath"
	"strings"
)

// CacheDir is <cache>/kigumi/mod/<name>@<version> with uppercase letters escaped as !x, so names differing only by case stay apart on case-insensitive filesystems.
func CacheDir(o CompilerOptions, r Require) (string, error) {
	if err := ValidModuleName(r.Name); err != nil {
		return "", err
	}
	if err := ValidVersion(r.Version); err != nil {
		return "", err
	}
	cache, err := o.cacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "kigumi", "mod", filepath.FromSlash(escapePath(r.Name))+"@"+escapePath(r.Version)), nil
}

func escapePath(s string) string {
	segs := strings.Split(s, "/")
	for i, seg := range segs {
		segs[i] = escapeSegment(seg)
	}
	return strings.Join(segs, "/")
}

// windowsReservedComponent are MS-DOS/Win32 device names CreateFile reserves at any path component; ValidModuleName allows them since it enforces grammar, not filesystem reserved names.
var windowsReservedComponent = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com0": true, "com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt0": true, "lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// escapeSegment escapes uppercase letters as !<lowercase> (keeping the cache injective across case) and prefixes a Windows-reserved base with !!, a sequence the per-letter rule can never itself produce.
func escapeSegment(seg string) string {
	var sb strings.Builder
	base, _, _ := strings.Cut(seg, ".")
	if windowsReservedComponent[strings.ToLower(base)] {
		sb.WriteString("!!")
	}
	for _, c := range seg {
		if c >= 'A' && c <= 'Z' {
			sb.WriteByte('!')
			sb.WriteRune(c + ('a' - 'A'))
			continue
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

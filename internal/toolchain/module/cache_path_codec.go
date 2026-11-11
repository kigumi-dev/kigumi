package module

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	buildconfig "kigumi/internal/toolchain/config"
)

// CacheModulesDir is the directory holding one fetched copy per module@version.
func CacheModulesDir(o buildconfig.CompilerOptions) (string, error) {
	cache, err := o.CacheRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "kigumi", "mod"), nil
}

// CacheDir is <cache>/kigumi/mod/<name>@<version> with uppercase letters escaped as !x, so names differing only by case stay apart on case-insensitive filesystems.
func CacheDir(o buildconfig.CompilerOptions, r Require) (string, error) {
	if err := ValidModuleName(r.Name); err != nil {
		return "", err
	}
	if err := ValidVersion(r.Version); err != nil {
		return "", err
	}
	if err := CheckVersionForName(r.Name, r.Version); err != nil {
		return "", err
	}
	mods, err := CacheModulesDir(o)
	if err != nil {
		return "", err
	}
	return filepath.Join(mods, filepath.FromSlash(escapePath(r.Name))+"@"+escapePath(r.Version)), nil
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

// DepDir resolves a required module's directory: a local override (root only), the root's vendored copy, or the fetched cache copy; missing is true when uncached.
func DepDir(o buildconfig.CompilerOptions, local Local, r Require, isRoot bool) (dir string, missing bool, err error) {
	if p, ok := local.Replaces[r.Name]; ok && isRoot {
		return p, false, nil
	}
	if isRoot && local.Vendor != "" {
		return vendorDir(local, r)
	}
	dir, err = CacheDir(o, r)
	if err != nil {
		return "", false, err
	}
	return dir, !CacheReady(dir), nil
}

// vendorDir is the checked-in copy of r. Its version is checked against the
// selection once selection finishes; a module missing from vendor/ is an
// inconsistent tree, not a cache miss, so falling back to the cache would
// hide it.
func vendorDir(local Local, r Require) (string, bool, error) {
	if err := ValidModuleName(r.Name); err != nil {
		return "", false, err
	}
	if _, ok := local.Vendored[r.Name]; !ok {
		return "", false, fmt.Errorf("%s is not in %s; run `kigumi mod vendor`", r.Name, filepath.Join(local.Vendor, VendorFileName))
	}
	dir := filepath.Join(local.Vendor, filepath.FromSlash(r.Name))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "", false, fmt.Errorf("%s is listed in %s but missing from %s; run `kigumi mod vendor`", r.Name, VendorFileName, local.Vendor)
	}
	return dir, false, nil
}

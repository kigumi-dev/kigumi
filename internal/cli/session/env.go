package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/toolchain/driver"
)

// CFlags returns the extra C compiler and linker arguments of KIGUMI_CFLAGS.
func CFlags() []string { return strings.Fields(os.Getenv("KIGUMI_CFLAGS")) }

// LocalOff reports KIGUMI_LOCAL=off, which ignores mod.local.kg.
func LocalOff() bool { return os.Getenv("KIGUMI_LOCAL") == "off" }

// PackageCache is KIGUMI_PKG_CACHE: a directory for the package cache, "off" to turn it off, else the default under the user cache dir.
func PackageCache() string {
	switch v := os.Getenv("KIGUMI_PKG_CACHE"); v {
	case "off":
		return ""
	case "":
		dir, _ := driver.DefaultPackageCache(CompilerOptions())
		return dir
	default:
		return v
	}
}

// UnitCheck is off with KIGUMI_PKG_UNITS=off, which makes `kigumi check` check the whole module instead of skipping packages whose cached outcome is clean, and every command check every std body.
func UnitCheck() bool { return os.Getenv("KIGUMI_PKG_UNITS") != "off" }

// NoAccel reports KIGUMI_NO_ACCEL, which runs std bodies in Kigumi.
func NoAccel() bool { return os.Getenv("KIGUMI_NO_ACCEL") != "" }

// VerifyPasses reports KIGUMI_VERIFY=all, which verifies the MIR after every pass instead of only the last.
func VerifyPasses() bool { return os.Getenv("KIGUMI_VERIFY") == "all" }

// ZigPath returns KIGUMI_ZIG, an explicit zig binary resolved before PATH.
func ZigPath() string { return os.Getenv("KIGUMI_ZIG") }

// ZigHome is KIGUMI_ZIG_HOME, else $XDG_DATA_HOME/kigumi/zig; "" defers to the store's own default.
func ZigHome() string {
	if home := os.Getenv("KIGUMI_ZIG_HOME"); home != "" {
		return home
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "kigumi", "zig")
	}
	return ""
}

// ZigStoreRoot is the managed zig store's directory, ZigHome resolved against the store's default.
func ZigStoreRoot() (string, error) { return CompilerOptions().ZigStoreRoot(ZigHome()) }

// Color decides ANSI colors for f: --color always/never settle it, else NO_COLOR turns it off, CLICOLOR_FORCE on,
// and a dumb terminal or a non-terminal stays plain.
func Color(mode string, f *os.File) (bool, error) {
	switch mode {
	case "", "auto":
	case "always":
		return true, nil
	case "never":
		return false, nil
	default:
		return false, fmt.Errorf("--color %q: want auto, always, or never", mode)
	}
	if _, off := os.LookupEnv("NO_COLOR"); off {
		return false, nil
	}
	if os.Getenv("CLICOLOR_FORCE") != "" && os.Getenv("CLICOLOR_FORCE") != "0" {
		return true, nil
	}
	if os.Getenv("TERM") == "dumb" || f == nil {
		return false, nil
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0, nil
}

// SecretFile returns the file named by KIGUMI_SECRET_<name>, or "".
func SecretFile(name string) string { return os.Getenv("KIGUMI_SECRET_" + name) }

// ToolEnv is the allowlisted environment a build.kg tool process gets: PATH, the locale and the temp directory.
func ToolEnv() []string {
	var env []string
	for _, name := range []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TMPDIR", "TEMP", "TMP", "SYSTEMROOT"} {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// CCEnv is the allowlisted environment of a --reproducible build's C compiler, which may not inherit os.Environ.
func CCEnv() []string { return ToolEnv() }

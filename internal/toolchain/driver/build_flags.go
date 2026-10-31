package driver

import "slices"

var noUnwindTables = []string{"-fno-unwind-tables", "-fno-asynchronous-unwind-tables"}

// lto reports a target whose link reads bitcode: an archive's embedder may not, and zig links Mach-O without LLD.
func lto(t Target) bool { return !t.Bare() && t.OS != "darwin" }

// codegenFlags are the optimization flags of a build. -O1 keeps zig cc out of Debug mode, which instruments C with
// UBSan calls freestanding has no runtime for.
func codegenFlags(t Target, sizeMin bool) []string {
	switch {
	case sizeMin && lto(t):
		return append([]string{"-Os", "-flto"}, noUnwindTables...)
	case sizeMin:
		return append([]string{"-Os"}, noUnwindTables...)
	case t.Bare() && t.Arch == "arm":
		return append([]string{"-O1"}, noUnwindTables...)
	}
	return []string{"-O1"}
}

// elfOS lists the hosted OSes whose executables are ELF, where section GC applies.
var elfOS = []string{"linux", "freebsd", "netbsd", "openbsd"}

// sectionGC reports a hosted executable whose linker drops the sections nothing references.
func sectionGC(t Target) bool { return !t.Bare() && slices.Contains(elfOS, t.OS) }

// BuildReport is what a build resolved that the build summary shows: the C library form and the size settings.
type BuildReport struct {
	Linkage   Linkage
	SizeMin   bool
	SectionGC bool
	LTO       bool
	Archive   bool
}

// SizeLine is the build summary's account of the size settings and what --size-min drops.
func (r BuildReport) SizeLine() string {
	gc, lto := "", ""
	if r.SectionGC {
		gc = ", section GC"
	}
	if r.LTO {
		lto = ", LTO"
	}
	switch {
	case r.SizeMin && r.Archive:
		return "minimised by --size-min (-Os, no unwind tables; the embedder's link strips): drops the unwind tables debuggers, profilers and crash backtraces use"
	case r.SizeMin:
		return "minimised by --size-min (-Os" + lto + gc + ", no unwind tables, stripped): drops symbol names and the unwind tables debuggers, profilers and crash backtraces use"
	}
	debug := ""
	if r.Linkage.Static {
		debug = ", symbol names kept and debug info dropped"
	}
	return "-O1" + gc + debug + " (--size-min trades symbols and unwind tables for size)"
}

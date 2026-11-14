package driver

import (
	"fmt"
	"slices"
	"strings"

	opdef "kigumi/internal/compiler/mir/ops"
	buildconfig "kigumi/internal/toolchain/config"
)

// LinkMode is the --link choice: how a hosted executable for the host target gets its C library.
type LinkMode string

const (
	LinkAuto    LinkMode = ""
	LinkStatic  LinkMode = "static"
	LinkDynamic LinkMode = "dynamic"
)

// ParseLinkMode reads a --link value; "auto" and "" are the default.
func ParseLinkMode(s string) (LinkMode, error) {
	switch s {
	case "", "auto":
		return LinkAuto, nil
	case "static":
		return LinkStatic, nil
	case "dynamic":
		return LinkDynamic, nil
	}
	return LinkAuto, fmt.Errorf("link %q: want auto, static or dynamic", s)
}

// Linkage is the C library form a hosted build resolved to and why, for the build summary.
type Linkage struct {
	Static bool
	// Triple is what the C compiler got; "" is the host's own C library.
	Triple string
	Reason string
}

func (l Linkage) String() string {
	switch {
	case l.Reason == "":
		return ""
	case l.Static:
		return "static single binary (" + l.Triple + "): " + l.Reason
	case l.Triple != "":
		return "as " + l.Triple + " selects: " + l.Reason
	}
	return "dynamic, host C library: " + l.Reason
}

var muslTriples = map[string]string{"amd64": "x86_64-linux-musl", "arm64": "aarch64-linux-musl", "riscv64": "riscv64-linux-musl", "386": "x86-linux-musl", "arm": "arm-linux-musleabihf"}

// loaderNeed names what this build calls that needs the dynamic loader, or "" when nothing does.
func loaderNeed(code compiled) string {
	if slices.Contains(code.Capabilities, opdef.CapHostDL.Macro()) {
		return "std/ffi/dl is reached and dlopen needs the dynamic loader"
	}
	for _, sym := range code.Foreign {
		if sym == "dlopen" || sym == "dlmopen" {
			return "extern(C) " + sym + " is called and needs the dynamic loader"
		}
	}
	return ""
}

// dynamicNeeds lists what this build reaches or links that a static musl executable cannot provide.
func dynamicNeeds(m *Module, code compiled, cflags, lflags []string) []string {
	var why []string
	if need := loaderNeed(code); need != "" {
		why = append(why, need)
	}
	for _, l := range m.Links {
		if l.Library != "" {
			why = append(why, "native Link lib"+l.Library+" is a host library")
			break
		}
	}
	if len(m.Sources) > 0 {
		why = append(why, "native Source files compile against the host's C headers")
	}
	if m.headers.hostOnly != "" {
		why = append(why, m.headers.hostOnly)
	}
	for _, f := range append(append([]string{}, cflags...), lflags...) {
		switch {
		case strings.HasPrefix(f, "-fsanitize"):
			why = append(why, f+" needs the host's sanitizer runtime")
		case f == "-DKIGUMI_RC_CHECK":
			why = append(why, "the RC checker relies on glibc's heap consistency checks")
		case strings.HasPrefix(f, "-l"), strings.HasSuffix(f, ".so"), strings.HasSuffix(f, ".a"), strings.HasSuffix(f, ".o"):
			why = append(why, "link flag "+f+" names a host-built library")
		default:
			continue
		}
		break
	}
	return why
}

// resolveLinkage picks the C library of a hosted build. An explicit triple decides and --link may only agree with it;
// the host default on Linux becomes static musl unless --link dynamic, a non-zig compiler, or dynamicNeeds keeps the host's.
func resolveLinkage(m *Module, t buildconfig.Target, mode LinkMode, zig bool, code compiled, cflags, lflags []string) (buildconfig.Target, Linkage, error) {
	if t.Bare() {
		if mode != LinkAuto {
			return t, Linkage{}, fmt.Errorf("--link %s applies to an executable; a freestanding or --sys none build is an archive", mode)
		}
		return t, Linkage{}, nil
	}
	if t.Triple != "" {
		static := t.StaticLibc()
		form := LinkDynamic
		if static || t.OS == "wasi" {
			form = LinkStatic
		}
		if mode != LinkAuto && mode != form {
			return t, Linkage{}, fmt.Errorf("--link %s contradicts --target %s, whose ABI means --link %s", mode, t.Triple, form)
		}
		if need := loaderNeed(code); static && need != "" {
			return t, Linkage{}, fmt.Errorf("--target %s links statically, but %s; use a -gnu target", t.Triple, need)
		}
		return t, Linkage{Triple: t.Triple, Static: static, Reason: "--target " + t.Triple}, nil
	}
	triple, ok := muslTriples[t.Arch]
	needs := dynamicNeeds(m, code, cflags, lflags)
	switch {
	case mode == LinkDynamic:
		return t, Linkage{Reason: "--link dynamic"}, nil
	case t.OS != "linux" || !ok:
		if mode == LinkStatic {
			return t, Linkage{}, fmt.Errorf("--link static needs a Linux host on a musl-supported architecture, not %s/%s", t.OS, t.Arch)
		}
		return t, Linkage{Reason: "no static C library for " + t.OS + "/" + t.Arch}, nil
	case !zig:
		if mode == LinkStatic {
			return t, Linkage{}, fmt.Errorf("--link static needs zig, the only C compiler kigumi uses that ships a static musl libc")
		}
		return t, Linkage{Reason: "the C compiler is not zig, which alone ships a static musl libc"}, nil
	case mode == LinkStatic && loaderNeed(code) != "":
		return t, Linkage{}, fmt.Errorf("--link static: %s, which a static executable cannot do", loaderNeed(code))
	case mode == LinkStatic && m.headers.hostOnly != "":
		return t, Linkage{}, fmt.Errorf("--link static: %s, so its constants and layouts would describe another C library", m.headers.hostOnly)
	case mode == LinkAuto && len(needs) > 0:
		return t, Linkage{Reason: strings.Join(needs, "; ")}, nil
	}
	linkTarget, err := t.WithTriple(triple)
	if err != nil {
		return t, Linkage{}, err
	}
	reason := "nothing reached needs the dynamic loader"
	if mode == LinkStatic {
		reason = "--link static"
	}
	return linkTarget, Linkage{Static: true, Triple: triple, Reason: reason}, nil
}

// artifactLink is the --link a build graph artifact gets: only executables for the host target take it, and the
// graph must hold one, since an archive or an artifact with its own target already fixes its C library.
func artifactLink(rg *ResolvedGraph, idx int, mode LinkMode) (LinkMode, error) {
	if mode == LinkAuto {
		return mode, nil
	}
	host := func(t buildconfig.Target) bool { return !t.Bare() && t.Triple == "" }
	if host(rg.Targets[idx]) {
		return mode, nil
	}
	if !slices.ContainsFunc(rg.Targets, host) {
		return mode, fmt.Errorf("--link %s applies to executables for the host target, and this build graph has none", mode)
	}
	return LinkAuto, nil
}

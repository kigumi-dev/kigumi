package driver

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	opdef "kigumi/internal/compiler/mir/ops"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
)

func TestResolveLinkage(t *testing.T) {
	host := buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix"}
	dl := compiled{Output: llgen.Output{Capabilities: []string{opdef.CapHostDL.Macro()}}}
	externDL := compiled{Output: llgen.Output{Foreign: []string{"dlopen", "printf"}}}
	musl := buildconfig.Target{Triple: "x86_64-linux-musl", OS: "linux", Arch: "amd64", Sys: "posix"}
	gnu := buildconfig.Target{Triple: "x86_64-linux-gnu", OS: "linux", Arch: "amd64", Sys: "posix"}
	cases := []struct {
		name    string
		m       Module
		t       buildconfig.Target
		mode    LinkMode
		notZig  bool
		code    compiled
		cflags  []string
		static  bool
		triple  string
		reason  string
		failing string
	}{
		{name: "default", t: host, static: true, triple: "x86_64-linux-musl", reason: "nothing reached needs the dynamic loader"},
		{name: "arm64", t: buildconfig.Target{OS: "linux", Arch: "arm64", Sys: "posix"}, static: true, triple: "aarch64-linux-musl", reason: "nothing reached"},
		{name: "forced static", t: host, mode: LinkStatic, static: true, triple: "x86_64-linux-musl", reason: "--link static"},
		{name: "forced dynamic", t: host, mode: LinkDynamic, reason: "--link dynamic"},
		{name: "dl", t: host, code: dl, reason: "std/dl is reached"},
		{name: "dl forced static", t: host, code: dl, mode: LinkStatic, failing: "cannot do"},
		{name: "extern dlopen", t: host, code: externDL, reason: "extern(C) dlopen is called"},
		{name: "extern dlopen forced static", t: host, code: externDL, mode: LinkStatic, failing: "extern(C) dlopen"},
		{name: "explicit musl with dl", t: musl, code: dl, failing: "links statically, but std/dl"},
		{name: "explicit musl with extern dlopen", t: musl, code: externDL, failing: "use a -gnu target"},
		{name: "native link", t: host, m: Module{Links: []module.Link{{Library: "z"}}}, reason: "native Link libz"},
		{name: "native source", t: host, m: Module{Sources: []string{"x.c"}}, reason: "native Source"},
		{name: "sanitizer", t: host, cflags: []string{"-fsanitize=thread"}, reason: "-fsanitize=thread"},
		{name: "rc checker", t: host, cflags: []string{"-DKIGUMI_RC_CHECK"}, reason: "RC checker"},
		{name: "host library flag", t: host, cflags: []string{"/tmp/libx.a"}, reason: "link flag /tmp/libx.a"},
		{name: "plain define", t: host, cflags: []string{"-DFOO=1", "-O2"}, static: true, triple: "x86_64-linux-musl", reason: "nothing reached"},
		{name: "not zig", t: host, notZig: true, reason: "not zig"},
		{name: "not zig forced static", t: host, notZig: true, mode: LinkStatic, failing: "needs zig"},
		{name: "darwin", t: buildconfig.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}, reason: "no static C library for darwin/arm64"},
		{name: "darwin forced static", t: buildconfig.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}, mode: LinkStatic, failing: "Linux host"},
		{name: "explicit gnu", t: buildconfig.Target{Triple: "x86_64-linux-gnu", OS: "linux", Arch: "amd64", Sys: "posix"}, triple: "x86_64-linux-gnu", reason: "--target x86_64-linux-gnu"},
		{name: "explicit musl", t: musl, static: true, triple: "x86_64-linux-musl", reason: "--target"},
		{name: "gnu triple with link static", t: gnu, mode: LinkStatic, failing: "contradicts --target x86_64-linux-gnu"},
		{name: "gnu triple with link dynamic", t: gnu, mode: LinkDynamic, triple: "x86_64-linux-gnu", reason: "--target x86_64-linux-gnu"},
		{name: "musl triple with link static", t: musl, mode: LinkStatic, static: true, triple: "x86_64-linux-musl", reason: "--target"},
		{name: "musl triple with link dynamic", t: musl, mode: LinkDynamic, failing: "means --link static"},
		{name: "wasi triple with link dynamic", t: buildconfig.Target{Triple: "wasm32-wasi", OS: "wasi", Arch: "wasm32", Sys: "posix"}, mode: LinkDynamic, failing: "contradicts"},
		{name: "header only the host translates", t: host, m: Module{headers: headerLibc{hostOnly: "Header x.h translates only against the host C library's headers"}}, reason: "Header x.h"},
		{name: "header only the host translates forced static", t: host, mode: LinkStatic, m: Module{headers: headerLibc{hostOnly: "Header x.h translates only"}}, failing: "--link static: Header x.h"},
		{name: "bare", t: buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "none"}},
		{name: "bare with mode", t: buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "none"}, mode: LinkDynamic, failing: "archive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, link, err := resolveLinkage(&c.m, c.t, c.mode, !c.notZig, c.code, c.cflags, nil)
			if c.failing != "" {
				if err == nil || !strings.Contains(err.Error(), c.failing) {
					t.Fatalf("err = %v, want one containing %q", err, c.failing)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if link.Static != c.static || link.Triple != c.triple || !strings.Contains(link.Reason, c.reason) {
				t.Errorf("linkage = %+v, want static=%v triple=%q reason containing %q", link, c.static, c.triple, c.reason)
			}
			if c.static && got.Triple != c.triple {
				t.Errorf("target triple = %q, want %q", got.Triple, c.triple)
			}
			if !c.static && got.Triple != c.t.Triple {
				t.Errorf("a non-static build changed the triple to %q", got.Triple)
			}
		})
	}
}

func TestLinkageAndSizeLines(t *testing.T) {
	static := Linkage{Static: true, Triple: "x86_64-linux-musl", Reason: "nothing reached needs the dynamic loader"}
	if got := static.String(); got != "static single binary (x86_64-linux-musl): nothing reached needs the dynamic loader" {
		t.Errorf("static line = %q", got)
	}
	if got := (Linkage{Reason: "--link dynamic"}).String(); got != "dynamic, host C library: --link dynamic" {
		t.Errorf("dynamic line = %q", got)
	}
	if got := (Linkage{}).String(); got != "" {
		t.Errorf("an archive's linkage line = %q, want none", got)
	}
	minimal := BuildReport{SizeMin: true, SectionGC: true, LTO: true}.SizeLine()
	for _, want := range []string{"-Os", "LTO", "section GC", "no unwind tables", "stripped", "backtraces"} {
		if !strings.Contains(minimal, want) {
			t.Errorf("--size-min line %q does not mention %s", minimal, want)
		}
	}
	if got := (BuildReport{SectionGC: true}).SizeLine(); !strings.HasPrefix(got, "-O1, section GC") {
		t.Errorf("default size line = %q", got)
	}
	if got := codegenFlags(buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "none"}, true); strings.Contains(strings.Join(got, " "), "-flto") {
		t.Errorf("an archive's --size-min flags %v carry -flto", got)
	}
	if got := codegenFlags(buildconfig.Target{OS: "darwin", Arch: "arm64", Sys: "posix"}, true); strings.Contains(strings.Join(got, " "), "-flto") {
		t.Errorf("a Mach-O --size-min build %v carries -flto, which zig's non-LLD Mach-O linker rejects", got)
	}
	if got := (BuildReport{SizeMin: true}).SizeLine(); strings.Contains(got, "LTO") {
		t.Errorf("a --size-min line without LTO claims it: %q", got)
	}
}

func TestArtifactLink(t *testing.T) {
	host := buildconfig.Target{OS: "linux", Arch: "amd64", Sys: "posix"}
	bare := buildconfig.Target{OS: "freestanding", Arch: "amd64", Sys: "none"}
	musl := buildconfig.Target{Triple: "x86_64-linux-musl", OS: "linux", Arch: "amd64", Sys: "posix"}
	mixed := &ResolvedGraph{Targets: []buildconfig.Target{bare, host, musl}}
	for idx, want := range []LinkMode{LinkAuto, LinkDynamic, LinkAuto} {
		got, err := artifactLink(mixed, idx, LinkDynamic)
		if err != nil || got != want {
			t.Errorf("artifact %d: link = %q, %v; want %q", idx, got, err, want)
		}
	}
	if _, err := artifactLink(&ResolvedGraph{Targets: []buildconfig.Target{bare, musl}}, 0, LinkStatic); err == nil || !strings.Contains(err.Error(), "has none") {
		t.Errorf("a graph with no host executable took --link static: %v", err)
	}
	if got, err := artifactLink(&ResolvedGraph{Targets: []buildconfig.Target{bare}}, 0, LinkAuto); err != nil || got != LinkAuto {
		t.Errorf("auto link = %q, %v", got, err)
	}
}

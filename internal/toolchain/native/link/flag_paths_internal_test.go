package link

import (
	"path/filepath"
	"reflect"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
)

func TestResolveFlagPaths(t *testing.T) {
	cwd := t.TempDir()
	o := buildconfig.CompilerOptions{Cwd: cwd}
	abs := func(p string) string { return filepath.Join(cwd, p) }
	in := []string{"-Iinc", "-I", "inc2", "-Llib", "-L", "lib2", "-isystem", "sys", "-Tscript.ld", "-T", "s2.ld", "extra.c", "obj.o", "libx.a", "link.ld", "-lm", "-DX=1", "-Wl,--gc-sections", "-I/abs", "/abs/y.c"}
	want := []string{"-I" + abs("inc"), "-I", abs("inc2"), "-L" + abs("lib"), "-L", abs("lib2"), "-isystem", abs("sys"), "-T" + abs("script.ld"), "-T", abs("s2.ld"), abs("extra.c"), abs("obj.o"), abs("libx.a"), abs("link.ld"), "-lm", "-DX=1", "-Wl,--gc-sections", "-I/abs", "/abs/y.c"}
	got, err := ResolveFlagPaths(o, in)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveFlagPaths = %q, %v\nwant %q", got, err, want)
	}
	if _, err := ResolveFlagPaths(buildconfig.CompilerOptions{}, []string{"-Iinc"}); err == nil {
		t.Error("a relative path with an empty Cwd must fail")
	}
	if got, err := ResolveFlagPaths(buildconfig.CompilerOptions{}, []string{"-O2", "-I/abs"}); err != nil || len(got) != 2 {
		t.Errorf("flags without relative paths need no Cwd: %q %v", got, err)
	}
}

func TestResolveFlagPathForms(t *testing.T) {
	cwd := t.TempDir()
	o := buildconfig.CompilerOptions{Cwd: cwd}
	abs := func(p string) string { return filepath.Join(cwd, p) }
	cases := []struct{ in, want []string }{
		{[]string{"-iquote", "q"}, []string{"-iquote", abs("q")}},
		{[]string{"-iquoteq"}, []string{"-iquote" + abs("q")}},
		{[]string{"-idirafter", "d", "-idirafter", "/d"}, []string{"-idirafter", abs("d"), "-idirafter", "/d"}},
		{[]string{"-include", "pre.h"}, []string{"-include", abs("pre.h")}},
		{[]string{"--sysroot=sr"}, []string{"--sysroot=" + abs("sr")}},
		{[]string{"--sysroot", "sr"}, []string{"--sysroot", abs("sr")}},
		{[]string{"--sysroot=/sr"}, []string{"--sysroot=/sr"}},
		{[]string{"-Wl,-T,s.ld"}, []string{"-Wl,-T," + abs("s.ld")}},
		{[]string{"-Wl,-rpath,lib"}, []string{"-Wl,-rpath,lib"}},
		{[]string{"-Wl,-rpath,$ORIGIN/lib"}, []string{"-Wl,-rpath,$ORIGIN/lib"}},
		{[]string{"-Wl,-rpath-link,lib"}, []string{"-Wl,-rpath-link," + abs("lib")}},
		{[]string{"-Xlinker", "-rpath", "-Xlinker", "$ORIGIN/../lib"}, []string{"-Xlinker", "-rpath", "-Xlinker", "$ORIGIN/../lib"}},
		{[]string{"-Xlinker", "-rpath", "-Xlinker", "a/b"}, []string{"-Xlinker", "-rpath", "-Xlinker", "a/b"}},
		{[]string{"-Xlinker", "-T", "-O2", "-Xlinker", "x"}, []string{"-Xlinker", "-T", "-O2", "-Xlinker", "x"}},
		{[]string{"-Xlinker", "--script=s.ld"}, []string{"-Xlinker", "--script=" + abs("s.ld")}},
		{[]string{"-isystem=foo", "-I=foo"}, []string{"-isystem=foo", "-I=foo"}},
		{[]string{"-Wl,-rpath,/lib"}, []string{"-Wl,-rpath,/lib"}},
		{[]string{"-Wl,--script=s.ld,--gc-sections"}, []string{"-Wl,--script=" + abs("s.ld") + ",--gc-sections"}},
		{[]string{"-Wl,--gc-sections", "-Wl,-z,noexecstack"}, []string{"-Wl,--gc-sections", "-Wl,-z,noexecstack"}},
		{[]string{"-Wl,-T", "-Wl,s.ld"}, []string{"-Wl,-T", "-Wl," + abs("s.ld")}},
		{[]string{"-Wl,-T", "-O2", "-Wl,s.ld"}, []string{"-Wl,-T", "-O2", "-Wl,s.ld"}},
		{[]string{"-Xlinker", "-T", "-Xlinker", "s.ld"}, []string{"-Xlinker", "-T", "-Xlinker", abs("s.ld")}},
		{[]string{"-Xlinker", "-rpath-link", "-Xlinker", "lib"}, []string{"-Xlinker", "-rpath-link", "-Xlinker", abs("lib")}},
		{[]string{"-Xlinker", "sub/x.map"}, []string{"-Xlinker", abs("sub/x.map")}},
		{[]string{"-Xlinker", "--gc-sections", "-Xlinker", "-z", "-Xlinker", "noexecstack"}, []string{"-Xlinker", "--gc-sections", "-Xlinker", "-z", "-Xlinker", "noexecstack"}},
		{[]string{"-Ttext=0x1000", "-Tdata=0x2000"}, []string{"-Ttext=0x1000", "-Tdata=0x2000"}},
		{[]string{"@args.rsp", "@/abs.rsp"}, []string{"@" + abs("args.rsp"), "@/abs.rsp"}},
		{[]string{"-O2", "-lm", "-DX=1", "-std=c11"}, []string{"-O2", "-lm", "-DX=1", "-std=c11"}},
	}
	for _, c := range cases {
		got, err := ResolveFlagPaths(o, c.in)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("ResolveFlagPaths(%q) = %q, %v\nwant %q", c.in, got, err, c.want)
		}
	}
}

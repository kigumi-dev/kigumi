package cheader

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// genParses asserts the generated package is Kigumi source the parser
// accepts: a header import that emits a dangling type name or a C name
// Kigumi reserves fails only here, not in Generate.
func genParses(t *testing.T, src string) {
	t.Helper()
	f := token.NewFile("gen.kg", []byte(src))
	tree := syntax.Parse(f)
	if len(tree.Diags) > 0 {
		t.Errorf("generated package does not parse:\n%s\n--- source ---\n%s", tree.Diags[0].Msg, src)
	}
}

// A struct that points at itself (or at a sibling declared later) must not
// drop: the record name is resolvable while its own fields are walked.
func TestSelfAndMutuallyRecursiveRecordsImport(t *testing.T) {
	full := `pub const struct_Tree = extern struct {
    left: [*c]struct_Tree = null,
    right: [*c]struct_Tree = null,
    v: c_int = @import("std").mem.zeroes(c_int),
};
pub const A = struct_A;
pub const B = struct_B;
pub const struct_B = extern struct {
    a: [*c]A = null,
    y: c_int = @import("std").mem.zeroes(c_int),
};
pub const struct_A = extern struct {
    b: [*c]struct_B = null,
    x: c_int = @import("std").mem.zeroes(c_int),
};
pub const Foo = struct_Foo;
pub const struct_Foo = extern struct {
    x: c_int = @import("std").mem.zeroes(c_int),
};
pub extern fn tree_sum(t: [*c]struct_Tree) c_int;
pub extern fn use_a(a: [*c]A) c_int;
pub extern fn use_foo(f: [*c]Foo) c_int;
pub const Tree = struct_Tree;
`
	src, skipped := Generate(full, "", ABIFor("linux", "amd64"))
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	for _, want := range []string{
		"type Tree layout(C) = {",
		"pub left *mut Tree",
		"type A layout(C) = {",
		"pub b *mut B",
		"type B layout(C) = {",
		"pub a *mut A",
		"type Foo layout(C) = {",
		"pub fn tree_sum(t: *mut Tree) -> i32",
		"pub fn use_a(a: *mut A) -> i32",
		"pub fn use_foo(f: *mut Foo) -> i32",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source missing %q:\n%s", want, src)
		}
	}
	if n := strings.Count(src, "type Foo"); n != 1 {
		t.Errorf("the forward typedef should coalesce into one declaration, got %d:\n%s", n, src)
	}
	genParses(t, src)
}

// A pre-registered record whose layout still cannot be expressed drops again,
// so a function taking a pointer to it is skipped instead of naming a type
// the package never declares.
func TestUnlayableRecordDropsItsReferences(t *testing.T) {
	full := `pub const struct_Bad = extern struct {
    a: [3]c_int = undefined,
};
pub extern fn use_bad(b: [*c]struct_Bad) c_int;
`
	src, skipped := Generate(full, "", ABIFor("linux", "amd64"))
	if strings.Contains(src, "Bad") {
		t.Errorf("a skipped record must not leak its name:\n%s", src)
	}
	joined := strings.Join(skipped, "\n")
	for _, want := range []string{"Bad: unsupported field", "use_bad: unsupported parameter type"} {
		if !strings.Contains(joined, want) {
			t.Errorf("skips missing %q:\n%s", want, joined)
		}
	}
}

// translate-c spells a packed/aligned record as per-field align(N); Kigumi
// has layout(C, packed) and layout(C, align: N) for the two shapes it can
// reproduce. pack(2)'s lowered field alignment has no Kigumi spelling.
func TestPackedAndAlignedRecordLayout(t *testing.T) {
	full := `pub const struct_P2 = extern struct {
    c: u8 align(1) = 0,
    x: c_int align(1) = 0,
};
pub const struct_A16 = extern struct {
    c: u8 align(16) = 0,
    x: c_int = @import("std").mem.zeroes(c_int),
};
pub const struct_PackTwo = extern struct {
    c: u8 align(2) = 0,
    x: c_int align(2) = 0,
};
`
	src, skipped := Generate(full, "", ABIFor("linux", "amd64"))
	if !strings.Contains(src, "type P2 layout(C, packed) = {") {
		t.Errorf("packed record missing its layout:\n%s", src)
	}
	if !strings.Contains(src, "type A16 layout(C, align: 16) = {") {
		t.Errorf("16-aligned record missing its layout:\n%s", src)
	}
	if strings.Contains(src, "PackTwo") {
		t.Errorf("pack(2) is not expressible and must be skipped:\n%s", src)
	}
	if joined := strings.Join(skipped, "\n"); !strings.Contains(joined, "PackTwo") {
		t.Errorf("pack(2) skip not reported:\n%s", joined)
	}
}

// A header may name a function-pointer typedef's parameters
// (`(*fn)(const char *s, void *p)`); the name is not part of the type.
func TestNamedFunctionPointerParameters(t *testing.T) {
	full := `pub const FnPtr2 = ?*const fn (s: [*c]const u8, p: ?*anyopaque) callconv(.c) c_int;
pub const FnPtr = ?*const fn (c_int, ...) callconv(.c) c_int;
`
	src, skipped := Generate(full, "", ABIFor("linux", "amd64"))
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	for _, want := range []string{
		"type FnPtr2 = alias extern(C) fn(*const u8, *mut u8) -> i32",
		"type FnPtr = alias extern(C) fn(i32, ...) -> i32",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source missing %q:\n%s", want, src)
		}
	}
	genParses(t, src)
}

// C allows Kigumi's keywords as identifiers; a record, a typedef or a
// function named that way is renamed and every reference follows it.
func TestKeywordTypeNamesAreRenamed(t *testing.T) {
	full := `pub const struct_fn = extern struct {
    next: [*c]struct_fn = null,
    x: c_int = @import("std").mem.zeroes(c_int),
};
pub const mut = c_int;
pub extern fn mut_add(a: mut, b: mut) c_int;
pub const @"type" = c_int;
pub const fn_t = struct_fn;
pub const @"fn" = struct_fn;
`
	src, skipped := Generate(full, "", ABIFor("linux", "amd64"))
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	for _, want := range []string{
		"type fn_ layout(C) = {",
		"pub next *mut fn_",
		"type type_ = alias i32",
		"type mut_ = alias i32",
		"type fn_t = alias fn_",
		"pub fn mut_add(a: mut_, b: mut_) -> i32",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source missing %q:\n%s", want, src)
		}
	}
	genParses(t, src)
}

// <stdlib.h> spells exit/abort as `noreturn`; Kigumi's Never is the same
// role, so the declarations import instead of vanishing.
func TestNoreturnResultsImport(t *testing.T) {
	full := `pub extern fn abort() noreturn;
pub extern fn exit(__status: c_int) noreturn;
pub extern fn die() noreturn;
`
	src, skipped := Generate(full, "", ABIFor("linux", "amd64"))
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %v", skipped)
	}
	for _, want := range []string{
		"pub fn abort() -> Never",
		"pub fn exit(__status: i32) -> Never",
		"pub fn die() -> Never",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("source missing %q:\n%s", want, src)
		}
	}
	genParses(t, src)
}

// zig translate-c is target-blind about plain `char` (always `u8`), so the
// importer rewrites the C source before translating it.
func TestRewritePlainChar(t *testing.T) {
	in := `#include <charset.h>
/* char in a comment */
// char in a line comment
#define CHAR_MACRO 'c'
typedef struct S { char c; unsigned char u; signed char s; } S;
char echo_char(char c);
const char *name(void);
`
	got := string(rewritePlainChar([]byte(in), false))
	for _, want := range []string{
		"#include <charset.h>",
		"/* char in a comment */",
		"#define CHAR_MACRO 'c'",
		"unsigned char u",
		"signed char s",
		"signed char c",
		"char echo_char(signed char c)",
		// A pointer's pointee stays u8: its signedness is not part of the
		// C ABI and Kigumi's byte/string convention is unsigned.
		"const char *name",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewrite missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "unsigned signed") || strings.Contains(got, "signed signed") {
		t.Errorf("rewrite doubled an explicit signedness:\n%s", got)
	}
	if unsignedGot := string(rewritePlainChar([]byte("char f(char c);\n"), true)); !strings.Contains(unsignedGot, "unsigned char f(unsigned char c)") {
		t.Errorf("unsigned-char target rewrite: %s", unsignedGot)
	}
}

// End to end: the header's plain char becomes i8 on a signed-char target and
// stays u8 where plain char is unsigned.
func TestPlainCharSignednessByTarget(t *testing.T) {
	zig := zigOrSkip(t)
	header := writeHeader(t, "char echo_char(char c);\nint char_as_int(char c);\ntypedef char CharAlias;\n")
	for _, c := range []struct {
		os, arch string
		want     string
	}{
		{"linux", "amd64", "pub fn char_as_int(c: i8) -> i32"},
		{"linux", "arm64", "pub fn char_as_int(c: u8) -> i32"},
	} {
		res, err := Import(Options{ZigPath: zig, HeaderPath: header, ABI: ABIFor(c.os, c.arch), CacheDir: t.TempDir()})
		if err != nil {
			t.Fatalf("%s/%s: %v", c.os, c.arch, err)
		}
		if !strings.Contains(res.Source, c.want) {
			t.Errorf("%s/%s: source missing %q\n--- source ---\n%s", c.os, c.arch, c.want, res.Source)
		}
	}
}

// A header whose plain char reaches a C helper must agree with the C oracle
// on the same target: (int)(char)200 is -56 where char is signed.
func TestPlainCharMatchesC(t *testing.T) {
	zig := zigOrSkip(t)
	cc := testZigCC(t)
	dir := t.TempDir()
	header := filepath.Join(dir, "ch.h")
	helper := filepath.Join(dir, "ch.c")
	main := filepath.Join(dir, "main.c")
	if err := os.WriteFile(header, []byte("int char_as_int(char c);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("#include \"ch.h\"\nint char_as_int(char c) { return (int)c; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("#include <stdio.h>\n#include \"ch.h\"\nint main(void) { printf(\"%d\\n\", char_as_int(200)); return 0; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "prog")
	out, err := exec.Command(cc, main, helper, "-o", exe).CombinedOutput()
	if err != nil {
		t.Fatalf("cc: %v\n%s", err, out)
	}
	want := "-56\n"
	if got, err := exec.Command(exe).CombinedOutput(); err != nil || string(got) != want {
		t.Fatalf("C oracle: %q %v, want %q", got, err, want)
	}
	res, err := Import(Options{ZigPath: zig, HeaderPath: header, ABI: ABIFor("linux", "amd64"), CacheDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Source, "pub fn char_as_int(c: i8) -> i32") {
		t.Errorf("imported signature disagrees with the C oracle:\n%s", res.Source)
	}
}

// testZigCC is the same C compiler the toolchain uses (`zig cc`), so the
// oracle does not depend on a host gcc being installed.
func testZigCC(t *testing.T) string {
	t.Helper()
	zig := zigOrSkip(t)
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "cc")
	script := "#!/bin/sh\nexec " + zig + " cc \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return wrapper
}

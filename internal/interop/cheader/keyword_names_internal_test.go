package cheader

import (
	"strings"
	"testing"
)

func TestKeywordNamesAreRenamed(t *testing.T) {
	full := `pub extern fn @"export"(@"type": c_int, @"fn": c_int, match: c_int) c_int;
pub extern fn match(in: c_int) void;
pub extern fn match_(a: c_int) void;
pub const move: c_int = 3;
pub const fail = 7;
pub const Rec = extern struct {
    @"pub": c_int = @import("std").mem.zeroes(c_int),
    is: c_int = @import("std").mem.zeroes(c_int),
};
`
	src, skipped := Generate(full, "", ABI{})
	for _, want := range []string{
		"fn export(type_: i32, fn_: i32, match_: i32) -> i32",
		"@linkName(\"match\")\n    pub fn match__(in_: i32) -> Void",
		"fn match_(a: i32) -> Void",
		"const move_: i32 = 3",
		"const fail_: ",
		"pub pub_ i32",
		"pub is_ i32",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated source lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, `@"`) {
		t.Errorf("generated source keeps a zig-quoted name:\n%s", src)
	}
	if len(skipped) != 0 {
		t.Errorf("unexpected skips: %v", skipped)
	}
}

func TestImportKeywordNamesFromAHeader(t *testing.T) {
	zig := zigOrSkip(t)
	header := writeHeader(t, `
typedef struct Pair { int type; int match; } Pair;
void params(int type, int pub, int defer, int test, int comptime, int var, int fn, int export);
int match(int in);
int fail(int move);
#define is 4
`)
	res, err := Import(Options{ZigPath: zig, HeaderPath: header, ABI: ABIFor("linux", "amd64"), CacheDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"pub fn params(type_: i32, pub_: i32, defer_: i32, test: i32, comptime_: i32, var: i32, fn_: i32, export: i32)",
		"@linkName(\"match\")\n    pub fn match_(in_: i32) -> i32",
		"@linkName(\"fail\")\n    pub fn fail_(move_: i32) -> i32",
		"pub type_ i32",
		"pub match_ i32",
		"pub const is_: i32 = 4",
	} {
		if !strings.Contains(res.Source, want) {
			t.Errorf("source missing %q\n--- source ---\n%s", want, res.Source)
		}
	}
	if strings.Contains(res.Source, `@"`) {
		t.Errorf("source keeps a zig-quoted name:\n%s", res.Source)
	}
}

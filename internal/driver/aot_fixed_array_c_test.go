package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const fixedArrayCSrc = `import ffi from std/ffi

pub type Frame layout(C) = {
    pub tag u8
    pub a FixedArray[u16, 3]
    pub b FixedArray[i32, 2]
    pub w FixedArray[f64, 2]
}

pub type Small layout(C) = {
    pub tag u8
    pub a FixedArray[u16, 3]
}

pub type Pair layout(C) = {
    pub w FixedArray[f32, 2]
}

extern(C) {
    fn small_get_val(s: Small, i: i32) -> i64
    fn small_make(seed: i32) -> Small
    fn pair_sum(p: Pair) -> f32
    fn pair_make(x: f32) -> Pair
    fn frame_size() -> usize
    fn frame_get(f: *const Frame, i: i32) -> i64
    fn frame_get_val(f: Frame, i: i32) -> i64
    fn frame_fill(f: *mut Frame) -> Unit
    fn frame_make(seed: i32) -> Frame
}

fn line(f: Frame) -> String {
    "${f.tag} ${f.a[0]} ${f.a[1]} ${f.a[2]} ${f.b[0]} ${f.b[1]} ${f.w[0]} ${f.w[1]}"
}

let mut a = FixedArray.filled[u16, 3](0)
a[0] = 1
a[1] = 300
a[2] = 65535
let mut b = FixedArray.filled[i32, 2](-7)
b[1] = 2147483647
let frame = Frame { tag: 9, a: a, b: b, w: FixedArray.filled[f64, 2](0.5) }
print "size ${ffi.sizeOf(&frame)} ${unsafe { frame_size() }}"
let cv = ffi.CValue.new(frame)
let mut byPtr = "ptr"
let mut byVal = "val"
let mut i: i32 = 0
for i < 8 {
    byPtr = "${byPtr} ${unsafe { frame_get(ffi.toConst(cv.ptr()), i) }}"
    byVal = "${byVal} ${unsafe { frame_get_val(frame, i) }}"
    i = i + 1
}
print byPtr
print byVal
unsafe {
    frame_fill(cv.ptr())
}
print "filled ${line(cv.get())}"
print "made ${line(unsafe { frame_make(5) })}"
let mut sa = FixedArray.filled[u16, 3](4)
sa[2] = 40000
let small = Small { tag: 3, a: sa }
let back = unsafe { small_make(11) }
print "small ${unsafe { small_get_val(small, 0) }} ${unsafe { small_get_val(small, 3) }} ${back.tag} ${back.a[0]} ${back.a[2]}"
let pair = Pair { w: FixedArray.filled[f32, 2](1.5) }
let made = unsafe { pair_make(2.0) }
print "pair ${unsafe { pair_sum(pair) }} ${made.w.get(0) || 0.0} ${made.w.get(1) || 0.0}"
`

const fixedArrayCHelper = `#include <stddef.h>
#include <stdint.h>
struct frame { uint8_t tag; uint16_t a[3]; int32_t b[2]; double w[2]; };
size_t frame_size(void) { return sizeof(struct frame); }
static int64_t field(const struct frame *f, int i) {
    switch (i) {
    case 0: return f->tag;
    case 1: case 2: case 3: return f->a[i - 1];
    case 4: case 5: return f->b[i - 4];
    default: return (int64_t)(f->w[i - 6] * 1000);
    }
}
int64_t frame_get(const struct frame *f, int i) { return field(f, i); }
int64_t frame_get_val(struct frame f, int i) { return field(&f, i); }
void frame_fill(struct frame *f) {
    f->tag = 200; f->a[0] = 7; f->a[1] = 8; f->a[2] = 9; f->b[0] = -1; f->b[1] = -2; f->w[0] = 1.25; f->w[1] = 2.5;
}
struct small { uint8_t tag; uint16_t a[3]; };
int64_t small_get_val(struct small s, int i) { return i == 0 ? s.tag : s.a[i - 1]; }
struct small small_make(int seed) { struct small s = { (uint8_t)seed, { seed, seed * 2, seed * 3 } }; return s; }
struct pair { float w[2]; };
float pair_sum(struct pair p) { return p.w[0] + p.w[1]; }
struct pair pair_make(float x) { struct pair p = { { x, x * 3 } }; return p; }
struct frame frame_make(int seed) {
    struct frame f = { (uint8_t)seed, { seed + 1, seed + 2, seed + 3 }, { -seed, seed * 100 }, { seed / 2.0, seed * 1.5 } };
    return f;
}
`

func TestAotFixedArrayCLayout(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fixedc\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(fixedArrayCSrc), 0o644)
	helper := filepath.Join(root, "helper.c")
	os.WriteFile(helper, []byte(fixedArrayCHelper), 0o644)
	t.Setenv("KIGUMI_CFLAGS", helper)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	want := "size 32 32\nptr 9 1 300 65535 -7 2147483647 500 500\nval 9 1 300 65535 -7 2147483647 500 500\n" +
		"filled 200 7 8 9 -1 -2 1.25 2.5\nmade 5 6 7 8 -5 500 2.5 7.5\nsmall 3 40000 11 11 33\npair 3 2 6\n"
	if string(out) != want {
		t.Errorf("got\n%s\nwant\n%s", out, want)
	}
}

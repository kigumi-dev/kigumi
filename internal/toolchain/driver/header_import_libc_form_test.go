package driver_test

import (
	"bytes"
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// regcomp and regexec are declared by hand: musl's regex_t has an array field the importer skips.
const libcFormMain = `import form from c/form
import ffi from std/ffi

extern(C) {
    fn regcomp(preg: *mut u8, pattern: *const u8, flags: i32) -> i32
    fn regexec(preg: *const u8, subject: *const u8, n: usize, matches: *mut form.regmatch_t, flags: i32) -> i32
}

let blank = form.regmatch_t { rm_so: 0, rm_eo: 0 }
let size = ffi.sizeOf(&blank)
let pattern = "b(c)"
let pat = ffi.CString.new(&pattern)?
let text = "abcd"
let subject = ffi.CString.new(&text)?
let re = unsafe { form.malloc(256) }
let matches = unsafe { ffi.toMut(ffi.cast[form.regmatch_t](ffi.toConst(form.malloc(size * 2)))) }
let compiled = unsafe { regcomp(re, pat.ptr(), form.REG_EXTENDED) }
let found = unsafe { regexec(ffi.toConst(re), subject.ptr(), 2, matches, 0) }
let group = unsafe { ffi.readRecord(ffi.offset(ffi.toConst(matches), size.toInt() || 0)) }
print "${form.BUFSIZ} ${form.FOPEN_MAX} ${size} ${compiled} ${found} ${group.rm_so} ${group.rm_eo}"
`

func headerModule(t *testing.T, header, main string) string {
	t.Helper()
	root := t.TempDir()
	mod := "Module {\n    name: \"libcform\"\n    kigumi: \"0.1\"\n}\n\nHeader {\n    path: \"libc_form.h\"\n    package: \"form\"\n}\n"
	for name, src := range map[string]string{"mod.kg": mod, "libc_form.h": header, "main.kg": main} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func interpOf(t *testing.T, exe string) string {
	t.Helper()
	f, err := elf.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			b := make([]byte, p.Filesz)
			p.ReadAt(b, 0)
			return strings.TrimRight(string(b), "\x00")
		}
	}
	return ""
}

func needLinuxZig(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") || testCompiler().CCompiler() == nil {
		t.Skip("needs a 64-bit Linux host and a C compiler")
	}
	if _, err := exec.LookPath("zig"); err != nil {
		t.Skip("no zig")
	}
}

// BUFSIZ, FOPEN_MAX and sizeof(regmatch_t) differ between musl and glibc and regexec writes through the imported
// layout, so each link form, and --emit-llvm under the same options, must see the headers of the C library it links.
func TestHeaderImportFollowsLinkedLibc(t *testing.T) {
	needLinuxZig(t)
	root := headerModule(t, "#include <stdio.h>\n#include <stdlib.h>\n#include <regex.h>\n", libcFormMain)
	const musl, glibc = "1024 1000 16 0 0 2 3\n", "8192 16 8 0 0 2 3\n"
	for _, c := range []struct {
		name   string
		opts   driver.BuildOptions
		static bool
		want   string
	}{
		{"default", driver.BuildOptions{}, true, musl},
		{"link static", driver.BuildOptions{Link: driver.LinkStatic}, true, musl},
		{"musl triple with link static", driver.BuildOptions{Link: driver.LinkStatic, Target: muslTarget(t)}, true, musl},
		{"link dynamic", driver.BuildOptions{Link: driver.LinkDynamic}, false, glibc},
	} {
		t.Run(c.name, func(t *testing.T) {
			var r driver.BuildReport
			c.opts.Report = &r
			exe := buildRootWith(t, root, c.opts)
			interp := interpOf(t, exe)
			if r.Linkage.Static != c.static || (interp == "") != c.static {
				t.Fatalf("linkage %+v with interpreter %q, want static=%v", r.Linkage, interp, c.static)
			}
			if !c.static && !strings.Contains(interp, "ld-linux") {
				t.Skipf("the host C library behind %s is not glibc", interp)
			}
			got := runExe(t, exe)
			if got != c.want {
				t.Errorf("output %q, want %q", got, c.want)
			}
			other := musl
			if c.want == musl {
				other = glibc
			}
			c.opts.Report = nil
			if consts, printed := emittedConsts(t, root, c.opts), strings.Fields(got); !consts[printed[0]] || !consts[printed[1]] || consts[strings.Fields(other)[0]] {
				t.Errorf("--emit-llvm constants %v, but the binary printed BUFSIZ %s and FOPEN_MAX %s", consts, printed[0], printed[1])
			}
		})
	}
}

var constNum = regexp.MustCompile(`@rt_const_num\(ptr @"[^"]+", i32 \d+, i32 \d+, i64 (-?\d+),`)

func emittedConsts(t *testing.T, root string, opts driver.BuildOptions) map[string]bool {
	t.Helper()
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.CompileWith(m, &stderr, opts)
	if err != nil || !ok {
		t.Fatalf("emit: %v\n%s", err, stderr.String())
	}
	consts := map[string]bool{}
	for _, sub := range constNum.FindAllStringSubmatch(ir, -1) {
		consts[sub[1]] = true
	}
	return consts
}

func muslTarget(t *testing.T) driver.Target {
	t.Helper()
	triple := map[string]string{"amd64": "x86_64-linux-musl", "arm64": "aarch64-linux-musl"}[runtime.GOARCH]
	target, err := driver.ParseTarget(testCompiler(), triple, "")
	if err != nil {
		t.Fatal(err)
	}
	return target
}

// A header musl's headers cannot translate keeps the build on the host C library and says so; --link static refuses it.
func TestHeaderImportGlibcOnlyHeaderStaysDynamic(t *testing.T) {
	needLinuxZig(t)
	if _, err := os.Stat("/usr/include/gnu/libc-version.h"); err != nil {
		t.Skip("the host has no glibc headers")
	}
	root := headerModule(t, "#include <gnu/libc-version.h>\n#include <stdio.h>\n", "import form from c/form\n\nprint \"${form.BUFSIZ}\"\n")
	var r driver.BuildReport
	exe := buildRootWith(t, root, driver.BuildOptions{Report: &r})
	if r.Linkage.Static || !strings.Contains(r.Linkage.Reason, "Header libc_form.h translates only against the host C library") {
		t.Errorf("linkage %+v, want the host C library with the header named", r.Linkage)
	}
	if got := runExe(t, exe); got != "8192\n" {
		t.Errorf("output %q", got)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	_, err = driver.BuildWith(m, filepath.Join(t.TempDir(), "prog"), &stderr, driver.BuildOptions{Link: driver.LinkStatic})
	if err == nil || !strings.Contains(err.Error(), "--link static: Header libc_form.h") {
		t.Errorf("--link static err = %v, want a refusal naming the header\n%s", err, stderr.String())
	}
}

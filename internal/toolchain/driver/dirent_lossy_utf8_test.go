package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const direntLossyUtf8Src = `import fs from std/fs

let files = host.files()
let dirPath = fs.Path.fromString("sub")?
match files.list(dirPath) {
    Ok(names) -> {
        if names.get(0) is Some(p) {
            print "name=${p.toString()}"
        } else {
            print "empty"
        }
    }
    Err(e) -> print "err: ${e.message()}"
}
`

// TestDirentLossyUtf8 covers the display side of B04: a directory entry name
// that isn't valid UTF-8 (an arbitrary byte string on Linux) must list with
// U+FFFD in place of the bad byte instead of building a String that fails its
// UTF-8 invariant. Path carries the exact bytes separately (see
// TestDirentBytesIdentity); VM reads them from os.ReadDir and native from
// readdir(3).
func TestDirentLossyUtf8(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names are UTF-16; a raw byte name is not supported there")
	}
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	sub := filepath.Join(root, "main", "sub")
	os.MkdirAll(sub, 0o755)
	// A single 0xFF byte: not a valid UTF-8 lead byte on its own, so it
	// becomes exactly one U+FFFD.
	invalidName := string([]byte{0xff})
	if err := os.WriteFile(filepath.Join(sub, invalidName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(direntLossyUtf8Src), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompilerAt(filepath.Join(root, "main")), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	want := "name=sub/�"
	t.Run("vm", func(t *testing.T) {
		t.Chdir(filepath.Join(root, "main"))
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main/main.kg"})
		if err != nil || code != 0 {
			t.Fatalf("vm: err %v exit %d\n%s", err, code, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("vm stdout = %q, want %q", got, want)
		}
	})

	t.Run("native", func(t *testing.T) {
		if testCompiler().CCompiler() == nil {
			t.Skip("no C compiler")
		}
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, buildErr.String())
		}
		cmd := exec.Command(exe)
		cmd.Dir = filepath.Join(root, "main")
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("stdout = %q, want %q", got, want)
		}
	})
}

const direntBytesIdentitySrc = `import fs from std/fs
import os from std/os

let files = host.files()
let dirPath = fs.Path.fromString("sub")?
let names = files.list(dirPath)?
print "count=${names.len()}"
for i in 0..names.len() {
    match names.get(i) {
        Some(p) -> {
            let osPath = p.toOsString()
            let st = files.stat(p)?
            print "entry bytes=${osPath.len()} unicode=${osPath.isUnicode()} text=${p.toString()} size=${st.size} content=${files.read(p)?}"
        }
        None -> print "missing"
    }
}
files.remove(names.get(1).unwrap())?
let left = files.list(dirPath)?
print "left=${left.len()} content=${files.read(left.get(0).unwrap())?}"

let rootPath = fs.Path.fromString(".")?
let rawDir = os.OsString.fromUnixBytes(b"./d-\xff")
let mut found = false
for entry in files.list(rootPath)? {
    if entry.toOsString().equals(&rawDir) {
        found = true
        print "dir=${files.isDir(entry)?} name=${entry.toString()}"
        let kids = files.list(entry)?
        print "kids=${kids.len()} content=${files.read(kids.get(0).unwrap())?}"
    }
}
print "found=${found}"
`

// TestDirentBytesIdentity covers B04: two raw names that render to the same
// lossy display text ("bad-\xff" and "bad-\uFFFD") must stay distinct Paths,
// so listing, stat and reading each returns its own file, and removing one
// leaves the other intact. The VM must carry the raw dirent bytes through
// Path as well as native readdir(3).
func TestDirentBytesIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file names are UTF-16; a raw byte name is not supported there")
	}
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	sub := filepath.Join(root, "main", "sub")
	os.MkdirAll(sub, 0o755)
	rawName := string([]byte{'b', 'a', 'd', '-', 0xff})
	uniName := string([]byte{'b', 'a', 'd', '-', 0xef, 0xbf, 0xbd})
	rawDirName := string([]byte{'d', '-', 0xff})
	writeEntries := func(t *testing.T) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(sub, rawName), []byte("raw"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, uniName), []byte("unicode"), 0o644); err != nil {
			t.Fatal(err)
		}
		rawDir := filepath.Join(root, "main", rawDirName)
		if err := os.MkdirAll(rawDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rawDir, "inside.txt"), []byte("nested"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeEntries(t)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(direntBytesIdentitySrc), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompilerAt(filepath.Join(root, "main")), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	// Sorted by raw bytes: 0xEF (the UTF-8 replacement character) precedes
	// 0xFF, so the Unicode-named file lists first.
	want := strings.Join([]string{
		"count=2",
		"entry bytes=11 unicode=true text=sub/bad-\ufffd size=7 content=unicode",
		"entry bytes=9 unicode=false text=sub/bad-\ufffd size=3 content=raw",
		"left=1 content=unicode",
		"dir=true name=./d-\ufffd",
		"kids=1 content=nested",
		"found=true",
	}, "\n")
	t.Run("vm", func(t *testing.T) {
		writeEntries(t)
		t.Chdir(filepath.Join(root, "main"))
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main/main.kg"})
		if err != nil || code != 0 {
			t.Fatalf("vm: err %v exit %d\n%s", err, code, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("vm stdout = %q, want %q", got, want)
		}
	})

	t.Run("native", func(t *testing.T) {
		if testCompiler().CCompiler() == nil {
			t.Skip("no C compiler")
		}
		writeEntries(t)
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, buildErr.String())
		}
		cmd := exec.Command(exe)
		cmd.Dir = filepath.Join(root, "main")
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("stdout = %q, want %q", got, want)
		}
	})
}

const pathNulRejectionSrc = `import fs from std/fs
import os from std/os

let nul = os.OsString.fromUnixBytes(b"a\x00b")
match fs.Path.fromOsString(&nul) {
    Ok(p) -> print "os accepted ${p.toString()}"
    Err(e) -> print "os ${e.message()}"
}
let text = String.fromBytes(b"a\x00b").unwrap()
match fs.Path.fromString(text) {
    Ok(p) -> print "text accepted ${p.toString()}"
    Err(e) -> print "text ${e.message()}"
}
let files = host.files()
let prefix = fs.Path.fromString("prefix")?
print "prefix ${files.read(prefix)?}"
`

// TestPathNulRejection covers B04's NUL rule: an OS name containing a NUL must
// be rejected while building the Path, so no file operation can truncate it at
// the NUL and address an unrelated prefix entry. The check runs for both entry
// forms and both engines.
func TestPathNulRejection(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	main := filepath.Join(root, "main")
	os.MkdirAll(main, 0o755)
	if err := os.WriteFile(filepath.Join(main, "prefix"), []byte("kept"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(main, "main.kg"), []byte(pathNulRejectionSrc), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompilerAt(main), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	want := strings.Join([]string{
		"os path contains NUL",
		"text path contains NUL",
		"prefix kept",
	}, "\n")
	t.Run("vm", func(t *testing.T) {
		t.Chdir(main)
		var out, errOut bytes.Buffer
		code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main/main.kg"})
		if err != nil || code != 0 {
			t.Fatalf("vm: err %v exit %d\n%s", err, code, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("vm stdout = %q, want %q", got, want)
		}
	})

	t.Run("native", func(t *testing.T) {
		if testCompiler().CCompiler() == nil {
			t.Skip("no C compiler")
		}
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, buildErr.String())
		}
		cmd := exec.Command(exe)
		cmd.Dir = main
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("stdout = %q, want %q", got, want)
		}
	})
}

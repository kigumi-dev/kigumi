package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
)

const hostEnvArgsUtf8Src = `let e = host.env("KIGUMI_UTF8_TEST_VAR")
match e {
    Some(s) -> print "env=Some(${s.len()})"
    None -> print "env=None"
}
let a = host.args().get(1)
match a {
    Some(s) -> print "arg=Some(${s.len()})"
    None -> print "arg=None"
}
`

// TestHostEnvArgsRejectInvalidUtf8 covers utf8-text-2: an environment
// variable or argv entry that isn't valid UTF-8 (arbitrary bytes on Unix)
// must fold into None, not build a String that fails its UTF-8 invariant.
func TestHostEnvArgsRejectInvalidUtf8(t *testing.T) {
	invalid := string([]byte{0xff, 0xfe, 0xfd})
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte(hostEnvArgsUtf8Src), 0o644)

	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("vm", func(t *testing.T) {
		t.Setenv("KIGUMI_UTF8_TEST_VAR", invalid)
		var out, errOut bytes.Buffer
		args := []string{"main/main.kg", invalid}
		code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, args)
		if err != nil || code != 0 {
			t.Fatalf("vm: err %v exit %d\nstderr: %s", err, code, errOut.String())
		}
		want := "env=None\narg=None\n"
		if out.String() != want {
			t.Errorf("vm stdout = %q, want %q (invalid UTF-8 must not reach String unvalidated)", out.String(), want)
		}
	})

	t.Run("valid_utf8_still_works", func(t *testing.T) {
		t.Setenv("KIGUMI_UTF8_TEST_VAR", "hello")
		var out, errOut bytes.Buffer
		args := []string{"main/main.kg", "hi"}
		code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, args)
		if err != nil || code != 0 {
			t.Fatalf("vm: err %v exit %d\nstderr: %s", err, code, errOut.String())
		}
		want := "env=Some(5)\narg=Some(2)\n"
		if out.String() != want {
			t.Errorf("vm stdout = %q, want %q", out.String(), want)
		}
	})

	t.Run("build", func(t *testing.T) {
		if testCompiler().CCompiler() == nil {
			t.Skip("no C compiler")
		}
		exe := filepath.Join(t.TempDir(), "prog")
		var buildErr bytes.Buffer
		if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
			t.Fatalf("build: %v\n%s", err, buildErr.String())
		}

		run := func(env []string, arg string) string {
			cmd := exec.Command(exe, arg)
			cmd.Dir = t.TempDir()
			cmd.Env = env
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
			}
			return out.String()
		}

		t.Run("invalid", func(t *testing.T) {
			got := run(append(os.Environ(), "KIGUMI_UTF8_TEST_VAR="+invalid), invalid)
			want := "env=None\narg=None\n"
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
		t.Run("valid", func(t *testing.T) {
			got := run(append(os.Environ(), "KIGUMI_UTF8_TEST_VAR=hello"), "hi")
			want := "env=Some(5)\narg=Some(2)\n"
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	})
}

const hostOsStringSrc = `fn report(label: String, found: os.OsString?, text: String?) -> Unit {
    match found {
        Some(o) -> {
            let lossy = o.toStringLossy()
            let unicode = o.isUnicode()
            let len = o.len()
            let strict = match o.intoString() {
                Ok(_) -> "ok"
                Err(back) -> "back=${back.len()}"
            }
            print "${label} len=${len} unicode=${unicode} get=${text is Some(_)} lossy=${lossy} strict=${strict}"
        }
        None -> print "${label} none"
    }
}
report("arg", host.args().getOsString(1), host.args().get(1))
report("env", host.envOsString("KIGUMI_UTF8_TEST_VAR"), host.env("KIGUMI_UTF8_TEST_VAR"))
report("missing", host.args().getOsString(2), host.args().get(2))
`

func TestHostEnvArgsLosslessOsString(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "main"), 0o755)
	os.WriteFile(filepath.Join(root, "main/main.kg"), []byte("import os from std/os\n"+hostOsStringSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, value, want string }{
		{"invalid", "\xff\xfe\xfd", "arg len=3 unicode=false get=false lossy=��� strict=back=3\n" +
			"env len=3 unicode=false get=false lossy=��� strict=back=3\nmissing none\n"},
		{"mixed", "a\xffb\xe3\x81", "arg len=5 unicode=false get=false lossy=a�b�� strict=back=5\n" +
			"env len=5 unicode=false get=false lossy=a�b�� strict=back=5\nmissing none\n"},
		{"valid", "héllo", "arg len=6 unicode=true get=true lossy=héllo strict=ok\n" +
			"env len=6 unicode=true get=true lossy=héllo strict=ok\nmissing none\n"},
	}
	for _, tc := range cases {
		t.Run("vm/"+tc.name, func(t *testing.T) {
			t.Setenv("KIGUMI_UTF8_TEST_VAR", tc.value)
			var out, errOut bytes.Buffer
			code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, []string{"main/main.kg", tc.value})
			if err != nil || code != 0 {
				t.Fatalf("vm: err %v exit %d\nstderr: %s", err, code, errOut.String())
			}
			if out.String() != tc.want {
				t.Errorf("vm stdout = %q, want %q", out.String(), tc.want)
			}
		})
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	exe := filepath.Join(t.TempDir(), "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	for _, tc := range cases {
		t.Run("native/"+tc.name, func(t *testing.T) {
			cmd := exec.Command(exe, tc.value)
			cmd.Dir = t.TempDir()
			cmd.Env = append(os.Environ(), "KIGUMI_UTF8_TEST_VAR="+tc.value)
			var out, errOut bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errOut
			if err := cmd.Run(); err != nil {
				t.Fatalf("run: %v\nstderr: %s", err, errOut.String())
			}
			if out.String() != tc.want {
				t.Errorf("native stdout = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

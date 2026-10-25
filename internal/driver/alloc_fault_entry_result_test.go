package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"kigumi/internal/driver"
)

const constantStaticSrc = `type Pt = {
    pub x Int
    pub y Int
}

type Cfg = {
    pub name String
    pub at Pt
    pub on Bool
}

fn mkCfg() -> Cfg {
    Cfg { name: "cfg", at: Pt { x: 3, y: 4 }, on: true }
}

static cfg: Cfg = comptime mkCfg()

type Boom = {
    pub n Int
}

fn Boom.message(self) -> String { "boom" }

fn check(c: Cfg) -> Void! {
    if c.name != "cfg" || c.at.x != 3 || !c.on {
        fail Boom { n: 1 }
    }
}

fn main() -> Void! {
    let c = cfg
    let d = c
    check(d)?
    check(cfg)?
}
`

func TestFaultInjectionEntryAndConstantStaticAllocateNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cases := []struct{ name, src string }{
		{"Void! main returning Ok", "fn main() -> Void! {}\n"},
		{"static record holding a String", constantStaticSrc},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			exe := filepath.Join(t.TempDir(), "prog")
			var buildErr bytes.Buffer
			ok, err := driver.BuildWith(loadTryConstructorModule(t, c.src), exe, &buildErr, driver.BuildOptions{CFlags: []string{"-DKIGUMI_FAULT_INJECT"}})
			if err != nil || !ok {
				t.Fatalf("build: %v\n%s", err, buildErr.String())
			}
			for n := 1; n <= 8; n++ {
				cmd := exec.Command(exe)
				cmd.Dir = t.TempDir()
				cmd.Env = append(os.Environ(), "KIGUMI_FAIL_ALLOC_AT="+strconv.Itoa(n))
				var errOut bytes.Buffer
				cmd.Stderr = &errOut
				if code := exitCodeOf(cmd.Run()); code != 0 || errOut.Len() != 0 {
					t.Errorf("refusing allocation %d: exit %d, stderr %q", n, code, errOut.String())
				}
			}
		})
	}
}

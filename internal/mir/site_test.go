package mir_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/testkit"
)

func TestSiteAtResolvesFileAndLine(t *testing.T) {
	std := testkit.LoadStd(t, "../../std")
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.txtar")
	src := "-- src --\n" +
		"fn main() -> Void! {\n" +
		"    let x = 1\n" +
		"    let y = 10 / (x - 1)\n" +
		"}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	res, _, _ := checkFixture(t, path, std)
	prog := mir.Build(res)

	mainFn := prog.Entry
	if mainFn == nil {
		t.Fatal("no entry function in the built program")
	}

	site := prog.EntitySite(mainFn)
	if site.Func != mainFn.Name {
		t.Errorf("EntitySite.Func = %q, want %q", site.Func, mainFn.Name)
	}
	if site.Line != 1 {
		t.Errorf("EntitySite.Line = %d, want 1 (fn main is the first line)", site.Line)
	}
	if site.File == "" {
		t.Error("EntitySite.File is empty")
	}

	for _, blk := range mainFn.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpBinary {
				s := prog.SiteAt(mainFn, in.Node)
				if s.Line != 3 {
					t.Errorf("division site line = %d, want 3", s.Line)
				}
				return
			}
		}
	}
	t.Fatal("no division instruction found to resolve a site for")
}

func TestSiteAtUnknownNode(t *testing.T) {
	var prog mir.Program
	if s := prog.SiteAt(nil, 0); s != (mir.Site{}) {
		t.Errorf("SiteAt(nil, 0) = %+v, want the zero Site", s)
	}
}

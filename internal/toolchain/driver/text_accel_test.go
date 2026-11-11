package driver_test

import (
	"bytes"
	"kigumi/internal/devkit/fixture"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// textAccelUse finds run fixtures that reach a std text function the VM accelerates in Go, directly or through std/json, std/net/http, std/fmt or std/cli.
var textAccelUse = regexp.MustCompile(`\.(trim|split|toUpper|toLower|tryTrim|trySplit|tryToUpper|tryToLower|contains|indexOf|lastIndexOf)\(|text\.|array\.(join|tryJoin)\(|StringBuilder|from std/(json|http|fmt|cli)`)

// arrayAccelUse finds run fixtures that reach an array copy body the VM accelerates in Go, directly or through Map.keys / values.
var arrayAccelUse = regexp.MustCompile(`\.(clone|tryClone|take|tryTake|reverse|tryReverse|concat|tryConcat|keys|values|tryKeys|tryValues)\(`)

// TestTextAcceleratorsMatchKigumiBodies runs those fixtures with the accelerators and with NoAccel, where the Kigumi bodies run, and wants the same run from both.
func TestTextAcceleratorsMatchKigumiBodies(t *testing.T) {
	checkAcceleratorsMatchBodies(t, textAccelUse, 20)
}

func TestArrayAcceleratorsMatchKigumiBodies(t *testing.T) {
	checkAcceleratorsMatchBodies(t, arrayAccelUse, 20)
}

func checkAcceleratorsMatchBodies(t *testing.T, use *regexp.Regexp, least int) {
	manifest := loadManifest(t)
	files, err := fixture.Corpus("../../..", "run")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	picked := 0
	for _, path := range fixture.ShardPaths(files) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fx := manifest.ByID(fixtureID(path))
		if fx == nil || !fx.Runs("vm") || !use.Match(src) {
			continue
		}
		picked++
		t.Run(filepath.Base(path), func(t *testing.T) {
			if !countsOpenFds(t, path) {
				t.Parallel()
			}
			root, wantOut, _, wantExit := loadRunFixture(t, path)
			run := func(noAccel bool) (string, string, int) {
				m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
				if err != nil {
					t.Fatal(err)
				}
				var out, errOut bytes.Buffer
				code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir(), NoAccel: noAccel}, &out, &errOut, []string{"main/main.kg", "one", "two"})
				if err != nil && noAccel && strings.Contains(err.Error(), "is not supported") {
					t.Skipf("the VM runs this fixture only through host accelerators: %v", err)
				}
				if err != nil {
					t.Fatalf("NoAccel=%v: %v", noAccel, err)
				}
				return out.String(), errOut.String(), code
			}
			bodyOut, bodyErr, bodyCode := run(true)
			accelOut, accelErr, accelCode := run(false)
			if bodyOut != wantOut || bodyCode != wantExit {
				t.Errorf("the Kigumi bodies print\n%s(exit %d), want\n%s(exit %d)", bodyOut, bodyCode, wantOut, wantExit)
			}
			if accelOut != bodyOut || accelErr != bodyErr || accelCode != bodyCode {
				t.Errorf("the accelerators print\n%s%s(exit %d), the bodies\n%s%s(exit %d)", accelOut, accelErr, accelCode, bodyOut, bodyErr, bodyCode)
			}
		})
	}
	if share := fixture.ShardShare(least); share > 0 && picked < share {
		t.Fatalf("picked only %d fixtures", picked)
	}
}

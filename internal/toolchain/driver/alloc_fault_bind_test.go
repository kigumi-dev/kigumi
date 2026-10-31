package driver_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const bindSweep = 24

var tickAgreeSources = map[string]string{
	"option": `fn byRef(a: &Int?) -> u64 {
    hash(a)
}

fn main() {
    let x = Some(1)
    let k = byRef(&x)
}
`,
	"method_value": `type T = {
    pub n Int
}

fn T.get(self, v: Int) -> Int {
    self.n + v
}

fn apply(f: fn(Int) -> Int, v: Int) -> Int {
    f(v)
}

fn main() {
    let t = T { n: 1 }
    let a = apply(t.get, 2)
}
`,
	"user_impl_1": `type W[A] = {
    pub a A
}

pure fn W[A: Hash].hash(self) -> u64 {
    1
}

pure fn W[A: Hash].equals(self, o: &W[A]) -> Bool {
    true
}

fn byRef(a: &W[Int]) -> u64 {
    hash(a)
}

fn main() {
    let x = W { a: 1 }
    let k = byRef(&x)
}
`,
	"user_impl_2": `type P[A, B] = {
    pub a A
    pub b B
}

pure fn P[A: Hash, B: Hash].hash(self) -> u64 {
    1
}

pure fn P[A: Hash, B: Hash].equals(self, o: &P[A, B]) -> Bool {
    true
}

fn byRef(a: &P[Int, Int]) -> u64 {
    hash(a)
}

fn main() {
    let x = P { a: 1, b: 2 }
    let k = byRef(&x)
}
`,
	"user_impl_3": `type R[A, B, C] = {
    pub a A
    pub b B
    pub c C
}

pure fn R[A: Hash, B: Hash, C: Hash].hash(self) -> u64 {
    1
}

pure fn R[A: Hash, B: Hash, C: Hash].equals(self, o: &R[A, B, C]) -> Bool {
    true
}

fn byRef(a: &R[Int, Int, Int]) -> u64 {
    hash(a)
}

fn main() {
    let x = R { a: 1, b: 2, c: 3 }
    let k = byRef(&x)
}
`,
}

func fixtureSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/sem/" + name + ".txtar")
	if err != nil {
		t.Fatal(err)
	}
	_, src, _ := strings.Cut(string(data), "-- src --\n")
	src, _, _ = strings.Cut(src, "\n-- ")
	return src
}

func loadSweepModule(t *testing.T, src, profile string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"bindsweep\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	target := driver.HostTarget()
	target.Profile = profile
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Target: target})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func vmSweepExits(t *testing.T, m *driver.Module) []int {
	t.Helper()
	return vmSweepExitsTo(t, m, bindSweep, false)
}

func nativeSweepExits(t *testing.T, m *driver.Module) []int {
	t.Helper()
	return nativeSweepExitsTo(t, m, bindSweep, false)
}

// Building a closure (a witness bind, a method value) and calling through it take the same refusable allocations on both backends.
func TestWitnessBindTicksAgree(t *testing.T) {
	t.Parallel()
	for name, src := range tickAgreeSources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadSweepModule(t, src, "")
			vmExits := vmSweepExits(t, m)
			if vmExits[0] == 0 {
				t.Fatalf("vm: the first allocation was not refused: %v", vmExits)
			}
			if native := nativeSweepExits(t, m); !slices.Equal(vmExits, native) {
				t.Errorf("fault sweep exits differ\nvm:     %v\nnative: %v", vmExits, native)
			}
		})
	}
}

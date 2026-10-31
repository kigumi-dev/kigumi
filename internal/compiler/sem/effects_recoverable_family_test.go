package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// familyCases pair every fallible std function with the plain form it came from: the try form passes the recoverable-only profile, the plain form is rejected.
var familyCases = []struct{ name, setup, try, plain string }{
	{"Array.empty", "", "_ = Array.tryEmpty[Int]()", "_ = Array.empty[Int]()"},
	{"Array.clone", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryClone()", "_ = a.clone()"},
	{"Array.cloneWith", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryCloneWith((x: &Int) => 1)", "_ = a.cloneWith((x: &Int) => 1)"},
	{"Map.empty", "import map from std/map\n\n", "_ = map.tryEmpty[Int, Int]()", "_ = map.empty[Int, Int]()"},
	{"StringBuilder.new", "import text from std/text\n\n", "_ = text.StringBuilder.tryNew()", "_ = text.StringBuilder.new()"},
	{"Array.fill", "", "_ = Array.tryFill(1, 2)", "_ = Array.fill(1, 2)"},
	{"Array.insertAt", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryInsertAt(0, 1)", "_ = a.insertAt(0, 1)"},
	{"Array.push", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryPush(1)", "a.push(1)"},
	{"Array.appendAll", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryAppendAll(Array.tryEmpty[Int]().unwrap())", "a.appendAll(Array.tryEmpty[Int]().unwrap())"},
	{"Array.take", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryTake(1)", "_ = a.take(1)"},
	{"Array.map", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryMap((x: &Int) => 1)", "_ = a.map((x: &Int) => 1)"},
	{"Array.filter", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryFilter((x: &Int) => true)", "_ = a.filter((x: &Int) => true)"},
	{"Array.reverse", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryReverse()", "_ = a.reverse()"},
	{"Array.slice", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.trySlice(0, 0)", "_ = a.slice(0, 0)"},
	{"Array.concat", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryConcat(Array.tryEmpty[Int]().unwrap())", "_ = a.concat(Array.tryEmpty[Int]().unwrap())"},
	{"Map.clone", "import map from std/map\n\nlet m = map.tryEmpty[Int, Int]().unwrap()\n", "_ = m.tryClone()", "_ = m.clone()"},
	{"Map.entries", "import map from std/map\n\nlet m = map.tryEmpty[Int, Int]().unwrap()\n", "_ = m.tryEntries()", "_ = m.entries()"},
	{"Map.keys", "import map from std/map\n\nlet m = map.tryEmpty[Int, Int]().unwrap()\n", "_ = m.tryKeys()", "_ = m.keys()"},
	{"Map.values", "import map from std/map\n\nlet m = map.tryEmpty[Int, Int]().unwrap()\n", "_ = m.tryValues()", "_ = m.values()"},
	{"Map.set", "import map from std/map\n\nlet mut m = map.tryEmpty[Int, Int]().unwrap()\n", "_ = m.trySet(1, 2)", "m.set(1, 2)"},
	{"String.concat", "let s = \"x\"\n", "_ = s.tryConcat(s)", "_ = s.concat(s)"},
	{"String.repeat", "let s = \"x\"\n", "_ = s.tryRepeat(2)", "_ = s.repeat(2)"},
	{"String.sliceBytes", "let s = \"x\"\n", "_ = s.trySliceBytes(0, 1)", "_ = s.sliceBytes(0, 1)"},
	{"String.trim", "let s = \"x\"\n", "_ = s.tryTrim()", "_ = s.trim()"},
	{"String.fromBytes", "let b = Bytes.tryZeros(1).unwrap()\n", "_ = String.tryFromBytes(b)", "_ = String.fromBytes(b)"},
	{"Bytes.zeros", "", "_ = Bytes.tryZeros(1)", "_ = Bytes.zeros(1)"},
	{"Bytes.fill", "", "_ = Bytes.tryFill(1, 1)", "_ = Bytes.fill(1, 1)"},
	{"Bytes.concat", "let b = Bytes.tryZeros(1).unwrap()\n", "_ = b.tryConcat(b)", "_ = b.concat(b)"},
	{"Bytes.slice", "let b = Bytes.tryZeros(1).unwrap()\n", "_ = b.trySlice(0, 1)", "_ = b.slice(0, 1)"},
	{"Bytes.fromArray", "let a = Array.tryEmpty[u8]().unwrap()\n", "_ = Bytes.tryFromArray(&a)", "_ = Bytes.fromArray(&a)"},
	{"Shared.new", "import {Shared} from std/alloc\n\n", "_ = Shared.tryNew(1)", "_ = Shared.new(1)"},
	{"Shared.get", "import {Shared} from std/alloc\n\nlet s = Shared.tryNew(1).unwrap()\n", "_ = s.tryGet()", "_ = s.get()"},
	{"Shared.downgrade", "import {Shared} from std/alloc\n\nlet s = Shared.tryNew(1).unwrap()\n", "_ = s.tryDowngrade()", "_ = s.downgrade()"},
	{"Arena.create", "import alloc from std/alloc\n\n", "_ = alloc.Arena.tryCreate()", "_ = alloc.Arena.create()"},
	{"StringBuilder.push", "import text from std/text\n\nlet mut b = text.StringBuilder.tryNew().unwrap()\n", "_ = b.tryPush(\"x\")", "b.push(\"x\")"},
	{"StringBuilder.pushByte", "import text from std/text\n\nlet mut b = text.StringBuilder.tryNew().unwrap()\n", "_ = b.tryPushByte(1)", "b.pushByte(1)"},
	{"String.split", "let s = \"x\"\n", "_ = s.trySplit(\",\")", "_ = s.split(\",\")"},
	{"String.chars", "let s = \"x\"\n", "_ = s.tryChars()", "_ = s.chars()"},
	{"String.toUpper", "let s = \"x\"\n", "_ = s.tryToUpper()", "_ = s.toUpper()"},
	{"String.toLower", "let s = \"x\"\n", "_ = s.tryToLower()", "_ = s.toLower()"},
	{"text.lines", "import text from std/text\n\n", "_ = text.tryLines(\"x\")", "_ = text.lines(\"x\")"},
	{"text.graphemes", "import text from std/text\n\n", "_ = text.tryGraphemes(\"x\")", "_ = text.graphemes(\"x\")"},
	{"text.replace", "import text from std/text\n\n", "_ = text.tryReplace(\"x\", \"a\", \"b\")", "_ = text.replace(\"x\", \"a\", \"b\")"},
	{"text.trimPrefix", "import text from std/text\n\n", "_ = text.tryTrimPrefix(\"x\", \"a\")", "_ = text.trimPrefix(\"x\", \"a\")"},
	{"text.trimSuffix", "import text from std/text\n\n", "_ = text.tryTrimSuffix(\"x\", \"a\")", "_ = text.trimSuffix(\"x\", \"a\")"},
	{"text.slice", "import text from std/text\n\n", "_ = text.trySlice(\"x\", 0, 1)", "_ = text.slice(\"x\", 0, 1)"},
	{"array.join", "import array from std/array\n\nlet items = Array.tryEmpty[String]().unwrap()\n", "_ = array.tryJoin(&items, \",\")", "_ = array.join(&items, \",\")"},
	{"array.sortedBy", "import array from std/array\n\nlet items = Array.tryEmpty[Int]().unwrap()\n", "_ = array.trySortedBy(&items, (a, b) => a.compareTo(b))", "_ = array.sortedBy(&items, (a, b) => a.compareTo(b))"},
	{"StringBuilder.freeze", "import text from std/text\n\nlet b = text.StringBuilder.tryNew().unwrap()\n", "_ = b.tryFreeze()", "_ = b.freeze()"},
	{"Array.filter scalar closure", "let mut a = Array.tryEmpty[Int]().unwrap()\n", "_ = a.tryFilter((x: &Int) => x * 2 > 7)", "_ = a.filter((x: &Int) => x * 2 > 7)"},
	{"array.sortedBy scalar closure", "import array from std/array\n\nlet items = Array.tryEmpty[Int]().unwrap()\n", "_ = array.trySortedBy(&items, (a, b) => (a * 3 - 1).compareTo(b * 3 - 1))", "_ = array.sortedBy(&items, (a, b) => (a * 3 - 1).compareTo(b * 3 - 1))"},
	{"debug", "let v = 1\n", "_ = tryDebug(&v)", "_ = debug(&v)"},
	{"mem.secureErase", "import mem from std/mem\n\nlet mut b = Bytes.tryZeros(1).unwrap()\n", "_ = mem.trySecureErase(&mut b)", "mem.secureErase(&mut b)"},
}

func TestRecoverableOnlyFamilyIsUniform(t *testing.T) {
	for _, c := range familyCases {
		t.Run(c.name, func(t *testing.T) {
			if got := checkRecoverableOnly(t, c.setup+c.try+"\n"); strings.Contains(got.Render(), "recoverable-only") {
				t.Errorf("the try form is rejected under recoverable-only:\n%s", got.Render())
			} else if got.HasErrors() {
				t.Errorf("the try form does not check:\n%s", got.Render())
			}
			if got := checkRecoverableOnly(t, c.setup+c.plain+"\n"); !strings.Contains(got.Render(), "recoverable-only") {
				t.Errorf("the plain form passes under recoverable-only:\n%s", got.Render())
			}
		})
	}
}

// A callback handing back a computed scalar boxes it on native, so its tryMap can still die on a refused allocation.
func TestRecoverableOnlyRejectsBoxedScalarCallbackResult(t *testing.T) {
	got := checkRecoverableOnly(t, "let mut a = Array.tryEmpty[Int]().unwrap()\n_ = a.tryMap((x: &Int) => x * 2 + 1)\n")
	if !strings.Contains(got.Render(), "recoverable-only") {
		t.Fatalf("a scalar-boxing callback must be rejected under recoverable-only:\n%s", got.Render())
	}
	ok := checkRecoverableOnly(t, "let mut a = Array.tryEmpty[Int]().unwrap()\n_ = a.tryMap((x: &Int) => 1)\n")
	if ok.HasErrors() {
		t.Fatalf("a constant-returning callback must pass:\n%s", ok.Render())
	}
}

// standaloneTry are fallible std functions with no plain sibling: building blocks the plain forms are written with.
var standaloneTry = map[string]bool{
	"array.Array.tryReserve": true, "array.Array.tryWithCapacity": true, "array.Array.tryCopyRange": true,
	"prelude.AllocError.wrap": true, "prelude.ByteBuffer.tryNew": true,
}

// A new try* function must come with its plain form (or be listed above), so the family stays uniform.
func TestEveryTryFunctionHasAPlainForm(t *testing.T) {
	res := sem.CheckWith(&sem.Module{Packages: testkit.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	names := map[string]bool{}
	var tries []string
	for id := range res.Entities {
		e := &res.Entities[id]
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 {
			continue
		}
		key := mir.StdKey(res, sem.EntityID(id), true)
		names[key] = true
		if len(e.Name) > 3 && strings.HasPrefix(e.Name, "try") && e.Name[3] >= 'A' && e.Name[3] <= 'Z' {
			tries = append(tries, key)
		}
	}
	if len(tries) < 30 {
		t.Fatalf("found only %d try functions; std not loaded?", len(tries))
	}
	for _, key := range tries {
		i := strings.LastIndexByte(key, '.')
		plain := key[:i+1] + strings.ToLower(key[i+4:i+5]) + key[i+5:]
		if !names[plain] && !standaloneTry[key] {
			t.Errorf("%s has no plain form %s", key, plain)
		}
	}
}

// A closure handing back a payload-free variant boxes an immortal, so it passes the profile; one with a payload allocates its box.
func TestRecoverableOnlyClosureResultBox(t *testing.T) {
	setup := "let a = Array.tryEmpty[Int]().unwrap()\n"
	if got := checkRecoverableOnly(t, setup+"_ = a.tryMap((x: &Int) => Ordering.Equal)\n"); got.HasErrors() {
		t.Errorf("a closure returning a payload-free variant is rejected:\n%s", got.Render())
	}
	if got := checkRecoverableOnly(t, setup+"_ = a.tryMap((x: &Int) => Some(1))\n"); !strings.Contains(got.Render(), "recoverable-only") {
		t.Errorf("a closure returning a variant with a payload passes:\n%s", got.Render())
	}
}

// The help line names the try sibling of the std convenience that was called.
func TestRecoverableOnlyHelpNamesTrySibling(t *testing.T) {
	got := checkRecoverableOnly(t, "let mut a = Array.tryEmpty[Int]().unwrap()\na.push(1)\n").Render()
	if !strings.Contains(got, "use the fallible form `tryPush` instead") {
		t.Errorf("the help does not name tryPush:\n%s", got)
	}
}

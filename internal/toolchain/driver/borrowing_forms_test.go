package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const borrowingFormsSrc = `import json from std/json
import test from std/test

type Bag = {
    pub name String
    pub items Array[Int]
}

let bag = Bag { name: "box", items: Array.of(1, 2, 3) }
let first = json.encodeRef(&bag) || "encode failed"
let second = json.encodeRef(&bag) || "encode failed"
print first
print "same=${first == second}"

let numbers = Array.of(4, 5)
print (json.encodeRef(&numbers) || "encode failed")
print (json.encodeRef(&numbers) || "encode failed")

let want = Bag { name: "box", items: Array.of(1, 2, 3) }
print "equal=${test.expectEqRef(&bag, &want) is Ok(_)}"
print "equal again=${test.expectEqRef(&bag, &want) is Ok(_)}"
let other = Bag { name: "box", items: Array.of(9) }
print "different=${test.expectEqRef(&bag, &other) is Ok(_)}"
print "still usable=${bag.items.len()}"
`

const borrowingFormsWant = `{"name":"box","items":[1,2,3]}
same=true
[4,5]
[4,5]
equal=true
equal again=true
different=false
still usable=3
`

func TestBorrowingEncodeAndExpectOnBothEngines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"borrowforms\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(borrowingFormsSrc), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code, err := driver.Run(m, &out, &errOut, nil); err != nil || code != 0 {
		t.Fatalf("vm: code=%d err=%v\n%s", code, err, errOut.String())
	}
	if out.String() != borrowingFormsWant {
		t.Errorf("vm output:\n%s\nwant:\n%s", out.String(), borrowingFormsWant)
	}
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler for the native half")
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	if got := runCstringNulProbe(t, exe); got != borrowingFormsWant {
		t.Errorf("native output:\n%s\nwant:\n%s", got, borrowingFormsWant)
	}
}

package driver_test

import (
	"testing"
	"time"
)

const worstCaseSearch = `import text from std/text

let hay = String.fromBytes(Bytes.fill(97, 40000))?
let needle = "${String.fromBytes(Bytes.fill(97, 19999))?}b"
print "contains ${hay.contains(needle)}"
print "find ${text.find(hay, needle) is Some(_)}"
print "indexOf ${hay.toBytes().indexOf(needle.toBytes()) is Some(_)}"
print "found at ${text.find("${hay}${needle}", needle) || 0}"
print "rfind ${text.rfind(hay, needle) is Some(_)} ${text.rfind("${needle}${hay}", needle) || 0}"
`

const worstCaseWant = "contains false\nfind false\nindexOf false\nfound at 40000\nrfind false 0\n"

func TestRunSubstringSearchIsLinear(t *testing.T) {
	if got := runGeneratedVM(t, worstCaseSearch, 5*time.Second); got != worstCaseWant {
		t.Fatalf("stdout %q", got)
	}
}

func TestAotSubstringSearchIsLinear(t *testing.T) {
	if got := runGeneratedAOT(t, worstCaseSearch, time.Second); got != worstCaseWant {
		t.Fatalf("stdout %q", got)
	}
}

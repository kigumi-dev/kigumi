package llgen

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// cDef is a parsed C function signature, parameter names stripped off.
type cDef struct {
	ret    string
	params []string
}

var defRe = regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_ ]*\*?)\s+(rt_[A-Za-z0-9_]+|display)\s*\(([^)]*)\)\s*\{`)

func parseCDefs(src string) map[string]cDef {
	out := map[string]cDef{}
	for _, m := range defRe.FindAllStringSubmatch(src, -1) {
		ret, name, rawParams := strings.TrimSpace(m[1]), m[2], strings.TrimSpace(m[3])
		var params []string
		if rawParams != "" && rawParams != "void" {
			for _, p := range strings.Split(rawParams, ",") {
				fields := strings.Fields(p)
				if len(fields) == 0 {
					continue
				}
				params = append(params, strings.Join(fields[:len(fields)-1], " "))
			}
		}
		out[name] = cDef{ret: ret, params: params}
	}
	return out
}

func cTypeMatches(t Type, c string) bool {
	switch t {
	case TPtr:
		return c == "val" || c == "adapter_fn" || strings.HasSuffix(c, "*")
	case TI32, TI1:
		return c == "int" || c == "uint32_t"
	case TI64:
		return c == "int64_t"
	case TDouble:
		return c == "double"
	case TVoid:
		return c == "void"
	}
	return false
}

// TestRuntimeFnsMatchC cross-checks the runtimeFns table against the C
// runtime it declares.
func TestRuntimeFnsMatchC(t *testing.T) {
	src, err := os.ReadFile("runtime/rt_core.c")
	if err != nil {
		t.Fatal(err)
	}
	defs := parseCDefs(string(src))
	if len(defs) < len(runtimeFns) {
		t.Fatalf("only parsed %d C definitions for %d table entries; regex drifted from rt_core.c", len(defs), len(runtimeFns))
	}
	for _, fn := range runtimeFns {
		def, ok := defs[fn.Name]
		if !ok {
			t.Errorf("%s: declared by llgen but not defined in rt_core.c", fn.Name)
			continue
		}
		if len(def.params) != len(fn.Params) {
			t.Errorf("%s: table has %d parameter(s), C definition has %d (%v)", fn.Name, len(fn.Params), len(def.params), def.params)
			continue
		}
		for i, p := range def.params {
			if !cTypeMatches(fn.Params[i], p) {
				t.Errorf("%s: parameter %d is %q in C, %s in the table", fn.Name, i, p, fn.Params[i])
			}
		}
		if !cTypeMatches(fn.Ret, def.ret) {
			t.Errorf("%s: returns %q in C, %s in the table", fn.Name, def.ret, fn.Ret)
		}
	}
}

// TestObjSlotMatchesC keeps the reserved immortal storage in step with the
// bound rt.h asserts struct obj against.
func TestObjSlotMatchesC(t *testing.T) {
	src, err := os.ReadFile("runtime/rt.h")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`#define RT_OBJ_SLOT (\d+)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("rt.h defines no RT_OBJ_SLOT")
	}
	if got := string(m[1]); got != strconv.Itoa(objSlotBytes) {
		t.Errorf("rt.h reserves %s bytes, llgen emits %d", got, objSlotBytes)
	}
	if !strings.Contains(header, "%rt_variant = type { ptr, ptr, i32, i32, "+objSlotType()+" }") {
		t.Errorf("the emitted %%rt_variant does not reserve %s:\n%s", objSlotType(), header)
	}
}

// The literal slots kg_init_immortals fills name their object kind by
// number, so the enum rt.h declares fixes these.
func TestObjectKindsMatchC(t *testing.T) {
	src, err := os.ReadFile("runtime/rt.h")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)enum \{ K_INT = 1,(.*?)\};`).FindSubmatch(src)
	if m == nil {
		t.Fatal("rt.h declares no object kind enum starting at K_INT = 1")
	}
	order := []string{"K_INT"}
	for _, name := range strings.Split(string(m[1]), ",") {
		if name = strings.TrimSpace(name); name != "" {
			order = append(order, name)
		}
	}
	for name, want := range map[string]int{"K_INT": kInt, "K_FLOAT": kFloat, "K_STR": kStr, "K_BYTES": kBytes, "K_CHAR": kChar} {
		got := slices.Index(order, name) + 1
		if got != want {
			t.Errorf("%s is %d in rt.h, %d in llgen", name, got, want)
		}
	}
}

package llgen_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	opdef "kigumi/internal/compiler/mir/ops"
)

var (
	stdIdCallRe = regexp.MustCompile(`@rt_std_id\(i32 (\d+),`)
	keyedCallRe = regexp.MustCompile(`call \S+ @(rt_std|rt_std_bind|rt_stditem_init)\(`)
)

// stdUseCases are programs whose emitted IR the runtime op selection must cover; a std function used as a
// value or as a witness keeps the keyed dispatch.
var stdUseCases = map[string]string{
	"plain":   "print \"hello\"\n",
	"text":    "let s = \"  a,b  \"\nprint \"${s.trim().split(\",\").len()}\"\n",
	"map":     "import map from std/map\nlet mut m = map.empty[String, Int]()\nm.set(\"a\", 1)\nprint \"${m.len()}\"\n",
	"fnvalue": "import math from std/math\nfn apply[A, R](f: fn(A) -> R, a: A) -> R { f(a) }\nprint \"${apply(math.sqrt, 4.0)}\"\n",
}

func TestStdOpMacrosCoverTheIR(t *testing.T) {
	for name, src := range stdUseCases {
		t.Run(name, func(t *testing.T) {
			out := llgen.EmitProgram(plannedProgram(t, src), llgen.EmitOptions{})
			if len(out.StdOps) == 0 || out.StdOps[0] != "KIGUMI_OP_SELECT" {
				t.Fatalf("StdOps = %v, want KIGUMI_OP_SELECT first", out.StdOps)
			}
			for _, m := range stdIdCallRe.FindAllStringSubmatch(out.IR, -1) {
				id, _ := strconv.Atoi(m[1])
				if want := llgen.OpMacro(opdef.Registry.Ops[id].Name); !slices.Contains(out.StdOps, want) {
					t.Errorf("the IR calls rt_std_id(%d) but StdOps lacks %s: %v", id, want, out.StdOps)
				}
			}
			keyed := keyedCallRe.MatchString(out.IR)
			dyn := slices.Contains(out.StdOps, "KIGUMI_STD_DYNAMIC")
			if dyn != keyed {
				t.Errorf("KIGUMI_STD_DYNAMIC = %v but the IR has a keyed std call = %v", dyn, keyed)
			}
			if want := name == "map" || name == "fnvalue"; dyn != want {
				t.Errorf("KIGUMI_STD_DYNAMIC = %v, want %v: %s", dyn, want, strings.Join(out.StdOps, " "))
			}
		})
	}
}

func TestStdOpOfMatchesSuffixesOnlyAfterADot(t *testing.T) {
	h := llgen.StdOpHeader(opdef.Registry)
	guard := strings.Index(h, "    if (dot) {\n")
	if guard < 0 {
		t.Fatal("rt_std_op_of does not guard the suffix matches on a dot, as mir.StdOpFor does")
	}
	end := guard + strings.Index(h[guard:], "\n    }\n")
	for _, op := range opdef.Registry.Ops {
		for _, s := range op.Suffixes {
			at := strings.Index(h, `!strcmp(m, "`+s+`")`)
			if at < guard || at > end {
				t.Errorf("suffix %q of %s is matched outside the dot guard", s, op.Name)
			}
		}
	}
}

package target

import (
	"strconv"
	"strings"
)

// dataLayouts are LLVM's datalayout strings as zig cc reports them, keyed by
// architecture; windowsDataLayouts holds the ones Windows changes. The first
// component is the byte order ("e" little, "E" big), which the table must
// agree with (BigEndian), and the p: entries give the pointer layout.
var dataLayouts = map[string]string{
	"amd64":    "e-m:e-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"386":      "e-m:e-p:32:32-p270:32:32-p271:32:32-p272:64:64-i128:128-f64:32:64-f80:32-n8:16:32-S128",
	"arm64":    "e-m:e-p270:32:32-p271:32:32-p272:64:64-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128-Fn32",
	"riscv64":  "e-m:e-p:64:64-i64:64-i128:128-n32:64-S128",
	"wasm32":   "e-m:e-p:32:32-p10:8:8-p20:8:8-i64:64-i128:128-n32:64-S128-ni:1:10:20",
	"arm":      "e-m:e-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"mips":     "E-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64",
	"mipsle":   "e-m:m-p:32:32-i8:8:32-i16:16:32-i64:64-n32-S64",
	"mips64":   "E-m:e-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128",
	"mips64le": "e-m:e-i8:8:32-i16:16:32-i64:64-i128:128-n32:64-S128",
	"ppc64":    "E-m:e-Fn32-i64:64-i128:128-n32:64-S128-v256:256:256-v512:512:512",
	"ppc64le":  "e-m:e-Fn32-i64:64-i128:128-n32:64-S128-v256:256:256-v512:512:512",
	"s390x":    "E-m:e-i1:8:16-i8:8:16-i64:64-f128:64-v128:64-a:8:16-n32:64",
	"loong64":  "e-m:e-p:64:64-i64:64-i128:128-n32:64-S128",
}

// windowsDataLayouts follows what zig cc reports for COFF targets: the mangling
// mode changes to the Windows one and arm64 also loses the ELF alignment
// entries, so a UEFI module's IR declares the same layout as its target.
var windowsDataLayouts = map[string]string{
	"amd64": "e-m:w-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:128-n8:16:32:64-S128",
	"arm64": "e-m:w-p270:32:32-p271:32:32-p272:64:64-p:64:64-i32:32-i64:64-i128:128-n32:64-S128-Fn32",
	"arm":   "e-m:w-p:32:32-Fi8-i64:64-v128:64:128-a:0:32-n32-S64",
	"386":   "e-m:x-p:32:32-p270:32:32-p271:32:32-p272:64:64-i64:64-i128:128-f80:32-n8:16:32-a:0:32-S32",
}

// DataLayout is the LLVM datalayout string of the target's C compiler.
func (l Layout) DataLayout() string {
	if l.OS == "windows" || l.OS == "uefi" {
		if s, ok := windowsDataLayouts[l.Arch]; ok {
			return s
		}
	}
	if s, ok := dataLayouts[l.Arch]; ok {
		return s
	}
	if l.PtrBits == 32 {
		return dataLayouts["386"]
	}
	return dataLayouts["amd64"]
}

var llvmDefaultInts = map[int]int{1: 8, 8: 8, 16: 16, 32: 32, 64: 32}

var llvmDefaultFloats = map[int]int{16: 16, 32: 32, 64: 64, 128: 128}

// ScalarAlign is the alignment in bytes of an integer or float of the given
// width inside a C struct on the target.
func (l Layout) ScalarAlign(float bool, bits int) int {
	return ScalarAlignIn(l.DataLayout(), float, bits)
}

// ScalarAlignIn reads that alignment from a datalayout string the way LLVM
// does: the exact width, else the next wider entry, else the widest one.
func ScalarAlignIn(dataLayout string, float bool, bits int) int {
	prefix, table := "i", map[int]int{}
	defaults := llvmDefaultInts
	if float {
		prefix, defaults = "f", llvmDefaultFloats
	}
	for k, v := range defaults {
		table[k] = v
	}
	for _, part := range strings.Split(dataLayout, "-") {
		if !strings.HasPrefix(part, prefix) {
			continue
		}
		fields := strings.Split(part[1:], ":")
		if len(fields) < 2 {
			continue
		}
		width, err1 := strconv.Atoi(fields[0])
		abi, err2 := strconv.Atoi(fields[1])
		if err1 == nil && err2 == nil {
			table[width] = abi
		}
	}
	if abi, ok := table[bits]; ok {
		return abi / 8
	}
	best, widest := 0, 0
	for w := range table {
		if w > bits && (best == 0 || w < best) {
			best = w
		}
		widest = max(widest, w)
	}
	if best == 0 {
		best = widest
	}
	return table[best] / 8
}

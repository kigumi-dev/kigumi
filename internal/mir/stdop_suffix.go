package mir

// stdOpSuffixes maps a trailing method name to its op, for primitives
// every eligible type defines the same way (Int8/u64.wrappingAdd, ...).
var stdOpSuffixes = map[string]string{
	"equals":             "EQUALS",
	"compareTo":          "COMPARE_TO",
	"hash":               "HASH",
	"wrappingAdd":        "WRAPPING_ADD",
	"wrappingSub":        "WRAPPING_SUB",
	"wrappingMul":        "WRAPPING_MUL",
	"wrappingShiftLeft":  "WRAPPING_SHL",
	"wrappingShiftRight": "WRAPPING_SHR",
	"rotateLeft":         "ROTATE_LEFT",
	"rotateRight":        "ROTATE_RIGHT",
}

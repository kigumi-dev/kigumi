package sem

var (
	cTruncateTarget   = defineCode("E2245", "truncate-target-type", "`{type}` cannot be the target of `truncate` on `{recv}`; truncate to an integer type no wider than `{recv}`, widen with a conversion such as `toU64()`, which keeps the value, or drop a float's fraction with `toInt()`")
	cTruncateNoTarget = defineCode("E2247", "truncate-target-missing", "`truncate` needs its target type spelled out; write `value.truncate[u8]()`, or use `toU8()` to keep the value when it fits")
)

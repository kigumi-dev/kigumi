package mir

type BuiltinNative uint8

const (
	NativeNone BuiltinNative = iota
	NativeRuntime
	NativeInline
)

type builtinArg uint8

const (
	argAny builtinArg = iota
	argString
	argInt
)

type builtinResult uint8

const (
	resAny builtinResult = iota
	resUnit
	resInt
	resBool
)

type BuiltinSpec struct {
	Name   string
	Args   []builtinArg
	Min    int
	Result builtinResult
	Iter   bool
	VM     bool
	Native BuiltinNative
}

var builtinSpecs = []BuiltinSpec{
	{Name: "print", Args: []builtinArg{argString}, Min: 1, Result: resUnit, VM: true, Native: NativeRuntime},
	{Name: "eprint", Args: []builtinArg{argString}, Min: 1, Result: resUnit, VM: true, Native: NativeRuntime},
	{Name: "panic", Args: []builtinArg{argString}, VM: true, Native: NativeRuntime},
	{Name: "host", VM: true, Native: NativeRuntime},
	{Name: "comptime"},
	{Name: "alloc.push", Args: []builtinArg{argAny}, Min: 1, VM: true, Native: NativeRuntime},
	{Name: "fixed.fill", Args: []builtinArg{argAny, argInt}, Min: 2, VM: true, Native: NativeRuntime},
	{Name: "iter.len", Args: []builtinArg{argAny}, Min: 1, Result: resInt, Iter: true, VM: true, Native: NativeRuntime},
	{Name: "iter.at", Args: []builtinArg{argAny, argInt}, Min: 2, Iter: true, VM: true, Native: NativeRuntime},
	{Name: "iter.at_ref", Args: []builtinArg{argAny, argInt}, Min: 2, Iter: true, VM: true, Native: NativeRuntime},
	{Name: "iter.at_move", Args: []builtinArg{argAny, argInt}, Min: 2, Iter: true, VM: true, Native: NativeRuntime},
	{Name: "direct.eq", Args: []builtinArg{argAny, argAny}, Min: 2, Result: resBool, VM: true, Native: NativeInline},
	{Name: "direct.hash", Args: []builtinArg{argAny}, Min: 1, Result: resInt, VM: true, Native: NativeInline},
}

var builtinByName = func() map[string]*BuiltinSpec {
	m := make(map[string]*BuiltinSpec, len(builtinSpecs))
	for i := range builtinSpecs {
		m[builtinSpecs[i].Name] = &builtinSpecs[i]
	}
	return m
}()

func BuiltinFor(name string) (BuiltinSpec, bool) {
	s, ok := builtinByName[name]
	if !ok {
		return BuiltinSpec{}, false
	}
	return *s, true
}

func BuiltinNames() []string {
	out := make([]string, len(builtinSpecs))
	for i, s := range builtinSpecs {
		out[i] = s.Name
	}
	return out
}

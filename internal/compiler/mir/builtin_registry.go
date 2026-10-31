package mir

type BuiltinNative uint8

const (
	NativeNone BuiltinNative = iota
	NativeRuntime
	NativeInline
)

// BuiltinSpec is one OpBuiltin the verifier and both backends know: its
// argument and result classes come from the registry's builtin lines.
type BuiltinSpec struct {
	Name   string
	Args   []ClassSet
	Min    int
	Result ClassSet
	Iter   bool
	VM     bool
	Native BuiltinNative
}

var builtinSpecs = Registry.Builtins

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

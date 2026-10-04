package sem

// StaticOwner says who owns a static's storage: Kigumi itself, or a
// foreign definition imported/exported across the C ABI.
type StaticOwner uint8

const (
	StaticOwned StaticOwner = iota
	StaticExtern
	StaticExport
)

// StaticInfo is a top-level `static`/`static mut` declaration.
type StaticInfo struct {
	Mut      bool
	Owner    StaticOwner
	Abi      string
	LinkName string
	Section  string
	Value    constValue
	State    uint8
	Metadata []MetaRef
}

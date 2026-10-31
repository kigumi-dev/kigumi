package fixture

// Reasons is the skip-reason catalog; manifests embed it inline so a
// reader never has to cross-reference this source file.
var Reasons = map[string]string{
	"native-only":    "the construct is native-only (C ABI, asm, ffi, dl); `kigumi build` runs it",
	"static-refusal": "this engine cannot run a construct the fixture needs (the VM's Supports check statically refuses it by name); only the engines left in mode run the fixture.",
}

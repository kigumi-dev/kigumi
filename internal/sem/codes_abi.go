package sem

// Diagnostic catalog, E188x: runtime ABI fingerprint.
var (
	RuntimeAbiMismatch = defineCode("E1880", "runtime-abi-mismatch", "{path} carries runtime ABI {got}, but this build links {want}; rebuild it with this toolchain and the same flags")
)

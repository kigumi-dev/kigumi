package sem

// Diagnostic catalog, E188x: runtime ABI fingerprint.
var (
	RuntimeAbiMismatch   = defineCode("E1880", "runtime-abi-mismatch", "{path} carries runtime ABI {got}, but this build links {want}; rebuild it with this toolchain and the same flags")
	RuntimeAbiUnreadable = defineCode("E1881", "runtime-abi-unreadable", "{path}: the runtime ABI sidecar cannot be read: {why}; rebuild the artifact with this toolchain, or remove the corrupt sidecar to link it as an opaque library")
)

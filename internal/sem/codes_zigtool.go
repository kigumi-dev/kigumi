package sem

// Diagnostic catalog, E156x: zig-pin resolution, built directly against mod.kg like E994.
var (
	ZigMissing         = defineCode("E1560", "zig-missing", "this module pins zig {want}, but no zig binary could be resolved ({detail}); install it, or point KIGUMI_ZIG at it")
	ZigVersionUnknown  = defineCode("E1561", "zig-version-unknown", "zig at {path} did not report its version ({detail}); this module pins zig {want} and needs a working zig to build natively")
	ZigVersionMismatch = defineCode("E1562", "zig-version-mismatch", "zig at {path} reports version {got}, but this module pins {want}; install the pinned version, or point KIGUMI_ZIG at a zig that reports it")
)

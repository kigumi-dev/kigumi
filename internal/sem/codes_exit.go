package sem

// Diagnostic catalog, E2207: process exit.
var cExitCodeRange = defineCode("E2207", "exit-code-range", "exit code {code} is outside 1 to 255; return from the entry normally for status 0")

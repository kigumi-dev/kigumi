package sem

// Diagnostic catalog, E132x: the async/blocking execution-agent boundary.
var (
	cAsyncDirectBlockStrict = defineCode("E1320", "async-direct-block", "`{name}` reaches a direct-blocking operation from its async body, which profile {profile} forbids; run it through a blocking execution provider or use an async-native operation instead")
	cAsyncDirectBlockWarn   = defineCode("W1320", "async-direct-block-warn", "`{name}` reaches a direct-blocking operation from its async body, occupying the current executor worker; run it through a blocking execution provider or use an async-native operation instead")
)

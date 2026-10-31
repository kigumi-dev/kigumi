package sem

// task cancellation: the `cancellation` context key.
var (
	cCancelKeyProvided = defineCode("E1440", "cancel-key-provided", "`cancellation` is bound by the task runtime at every suspension point and cannot be supplied by `{form}`; read it directly instead")
)

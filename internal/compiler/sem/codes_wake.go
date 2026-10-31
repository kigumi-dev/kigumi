package sem

// task wake from outside the executor.
var (
	cForeignWake = defineCode("E1780", "foreign-wake", "`{name}` is exported to C and reaches `Task.cancel`: a C caller may run it on a thread the executor does not know, which cannot wake a task safely; call `cancel` from inside a task instead")
)

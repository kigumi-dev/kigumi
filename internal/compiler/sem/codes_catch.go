package sem

// panic containment of spawned tasks.
var (
	cTaskPanicFatal = defineCode("W1860", "task-panic-fatal", "this program spawns tasks for `{target}`, where a panic in a task cannot be contained and ends the whole process; build for a target with panic containment or keep panics out of spawned tasks")
)

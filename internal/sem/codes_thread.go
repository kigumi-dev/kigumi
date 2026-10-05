package sem

// Diagnostic catalog, E146x: the multi-thread executor's target and profile gate.
var (
	cThreadPoolTarget  = defineCode("E1460", "thread-pool-target", "`{name}` needs OS worker threads, which this target has none of; drive the task tree with `task.currentThreadExecutor()` instead")
	cThreadPoolProfile = defineCode("E1461", "thread-pool-profile", "`{name}` needs OS worker threads, which profile {profile} forbids; drive the task tree with `task.currentThreadExecutor()` instead")
	cHandoffBound      = defineCode("E1462", "handoff-executor-bound", "this future is bound to one executor by `{type}` and cannot be handed to another; start it on the running task's own executor with `task.spawn`")
)

package sem

// async execution: suspension points, task lifetime and executors.
var (
	cBorrowAcrossAwait = defineCode("E1290", "borrow-across-await", "a borrow cannot be held across an `await`; end the borrow before the suspension point, or await an owned value")
	cRootBridgeInTask  = defineCode("E1291", "executor-cannot-host", "`runToCompletion` cannot run inside an `async fn`: this executor cannot host a task already bound to the running one; `await` the future instead")
	cExportAsync       = defineCode("E1292", "export-async", "`{name}` is exported to C and cannot be `async`: a C caller has no executor to drive the Future; export a synchronous wrapper that runs it to completion")
)

// Task handles stay inside the task that spawned them.
var (
	cTaskHandleReturned = defineCode("E2203", "task-handle-returned", "`{type}` holds a `Task` handle and cannot leave the task that spawned it as {where}; await the task there and pass on its result")
	cTaskHandleStored   = defineCode("E2204", "task-handle-stored", "`{type}` holds a `Task` handle and cannot be stored in `{where}`, which may outlive the task that spawned it; keep the handle in a local, record or array of that task")
	cTaskHandleCaptured = defineCode("E2205", "task-handle-captured", "`{name}` holds a `Task` handle and cannot be captured by a closure that may outlive the task that spawned it; await the task first, or pass the closure to a parameter that does not keep it")
)

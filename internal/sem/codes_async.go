package sem

// async execution: suspension points, task lifetime and executors.
var (
	cBorrowAcrossAwait = defineCode("E1290", "borrow-across-await", "a borrow cannot be held across an `await`; end the borrow before the suspension point, or await an owned value")
	cRootBridgeInTask  = defineCode("E1291", "executor-cannot-host", "`runToCompletion` cannot run inside an `async fn`: this executor cannot host a task already bound to the running one; `await` the future instead")
	cExportAsync       = defineCode("E1292", "export-async", "`{name}` is exported to C and cannot be `async`: a C caller has no executor to drive the Future; export a synchronous wrapper that runs it to completion")
)

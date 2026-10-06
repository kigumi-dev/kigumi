package sem

// Diagnostic catalog, E1330: the `mut self` receiver.
var cAsyncSelfAssign = defineCode("E1330", "async-self-assign", "`{name}` assigns `self` as a whole in an `async` body, where the caller's storage is out of reach by the time it runs; assign self's fields instead, or make the method synchronous")

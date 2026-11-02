package sem

var cSliceReadOnly = defineCode("E2236", "slice-read-only", "`{name}` is {where}, a new array that shares only the elements of `{source}`, so it cannot be {what}; reach it through `{source}` instead")

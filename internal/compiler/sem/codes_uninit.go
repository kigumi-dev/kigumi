package sem

var cUninitStorageAlloc = defineCode("E2246", "uninit-storage-allocates", "`{name}` needs the allocator, since this implementation keeps `{type}` in heap cells; it is unavailable in noalloc code, under the recoverable-only profile, and outside an `allocator` block in an `--allocator none` build")

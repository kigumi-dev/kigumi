package sem

// Diagnostic catalog, E2242: test blocks.
var cTestNameDuplicate = defineCode("E2242", "test-name-duplicate", "a test named {name} already exists in this package; give each test block its own name so its stable id is unique")

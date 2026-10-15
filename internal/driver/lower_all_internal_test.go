package driver

// LowerAll makes m lower every body, reachable or not, the way every build did before Build skipped unreached std and dependency bodies.
func LowerAll(m *Module) { m.lowerAll = true }

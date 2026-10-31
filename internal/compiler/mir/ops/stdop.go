package ops

// StdOpNames names, in id order, every op a bodiless std function can
// resolve to; the order is the registry's and the native runtime's `enum std_op`.
var StdOpNames = func() []string {
	names := make([]string, len(Registry.Ops))
	for i, op := range Registry.Ops {
		names[i] = op.Name
	}
	return names
}()

// providerOps lists the ops needing a Target/Capability (m.host); every
// other op is a PrimitiveId, the same on every backend.
var providerOps = func() map[string]bool {
	m := map[string]bool{}
	for _, op := range Registry.Ops {
		if op.Provider {
			m[op.Name] = true
		}
	}
	return m
}()

// stdOpKeys maps a std key ("std/"-stripped) to the op answering it; keys
// sharing an op share a runtime case.
var stdOpKeys = func() map[string]string {
	m := map[string]string{}
	for _, op := range Registry.Ops {
		for _, key := range op.Keys {
			m[key] = op.Name
		}
	}
	return m
}()

// stdOpSuffixes maps a trailing method name to its op, for primitives
// every eligible type defines the same way (Int8/u64.wrappingAdd, ...).
var stdOpSuffixes = func() map[string]string {
	m := map[string]string{}
	for _, op := range Registry.Ops {
		for _, s := range op.Suffixes {
			m[s] = op.Name
		}
	}
	return m
}()

// opCapability omits ops every target provides without a capability.
var opCapability = func() map[string]Capability {
	m := map[string]Capability{}
	for _, op := range Registry.Ops {
		if op.Cap != "" {
			m[op.Name] = op.Cap
		}
	}
	return m
}()

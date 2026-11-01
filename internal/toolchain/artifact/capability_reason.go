package artifact

// CapabilityReason is the sidecar record of why an artifact needs Capability.
type CapabilityReason struct {
	Capability string `json:"capability"`
	// Op is the std op that draws on Capability; empty when RequiredBy implies it.
	Op         string `json:"op,omitempty"`
	RequiredBy string `json:"requiredBy,omitempty"`
	// RootKind and Path describe the first reaching site, Sites[0].
	RootKind string           `json:"rootKind"`
	Path     []string         `json:"path"`
	Sites    []CapabilitySite `json:"sites"`
	Provided bool             `json:"provided"`
	Why      string           `json:"why,omitempty"`
}

// CapabilitySite is one function that reaches the op, with how many of its call sites do.
type CapabilitySite struct {
	RootKind string   `json:"rootKind"`
	Path     []string `json:"path"`
	Calls    int      `json:"calls"`
}

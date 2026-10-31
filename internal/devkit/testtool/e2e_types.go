package testtool

import "time"

// E2EFormatVersion versions E2EEvent's JSON; a consumer within one version ignores fields it does not know.
const E2EFormatVersion = 1

// E2EEvent is one phase measurement from RunE2E, uploaded as a CI artifact to observe, never to gate on.
type E2EEvent struct {
	FormatVersion  int       `json:"formatVersion"`
	Kind           string    `json:"kind"`
	Phase          string    `json:"phase"`
	Module         string    `json:"module"`
	Timestamp      time.Time `json:"timestamp"`
	DurationMS     float64   `json:"durationMs"`
	MaxRSSKiB      int64     `json:"maxRssKiB,omitempty"`
	ArtifactBytes  int64     `json:"artifactBytes,omitempty"`
	OK             bool      `json:"ok"`
	Error          string    `json:"error,omitempty"`
	OS             string    `json:"os"`
	Arch           string    `json:"arch"`
	NumCPU         int       `json:"numCpu"`
	ToolchainBuild string    `json:"toolchainCommit,omitempty"`
}

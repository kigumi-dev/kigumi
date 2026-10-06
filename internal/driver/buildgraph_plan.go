package driver

import (
	"encoding/json"

	"kigumi/internal/buildgraph"
	"kigumi/internal/schema"
	"kigumi/internal/version"
)

// PlanJSON is `kigumi build --plan`: the resolved graph as data, before any
// tool runs; p's own derive provenance rides along, since plan time never compiles a target artifact.
func PlanJSON(rg *ResolvedGraph, p *BuildProgram) string {
	type artifact struct {
		*buildgraph.Artifact
		Resolved    Target              `json:"resolved"`
		Fingerprint ArtifactFingerprint `json:"fingerprint"`
	}
	// resolvedFramework adds the winning descriptor's directory to the ref.
	type resolvedFramework struct {
		buildgraph.FrameworkRef
		Provider string `json:"provider,omitempty"`
	}
	plan := struct {
		FormatVersion    int                        `json:"formatVersion"`
		Toolchain        version.Provenance         `json:"toolchain"`
		NativeToolchain  []string                   `json:"nativeToolchain,omitempty"`
		Request          buildgraph.Request         `json:"request"`
		Frameworks       []resolvedFramework        `json:"frameworks"`
		Secrets          []buildgraph.SecretRef     `json:"secrets"`
		Files            []buildgraph.FileRef       `json:"files"`
		ToolSteps        []buildgraph.ToolStep      `json:"toolSteps"`
		Artifacts        []artifact                 `json:"artifacts"`
		Order            []string                   `json:"order"`
		DeriveProvenance []schema.GeneratedArtifact `json:"deriveProvenance,omitempty"`
	}{FormatVersion: version.BuildPlanFormatVersion, Toolchain: version.Current(), NativeToolchain: CCompiler(), Request: rg.Graph.Request, Secrets: rg.Graph.Secrets, Files: rg.Graph.Files, ToolSteps: rg.Graph.ToolSteps, Artifacts: []artifact{}, Order: rg.Order}
	if p != nil {
		plan.DeriveProvenance = schema.BuildProvenance(p.Res, nil)
	}
	for i, f := range rg.Graph.Frameworks {
		provider := ""
		if i < len(rg.Frameworks) {
			provider = rg.Frameworks[i].Dir
		}
		plan.Frameworks = append(plan.Frameworks, resolvedFramework{FrameworkRef: f, Provider: provider})
	}
	for i, a := range rg.Graph.Artifacts {
		plan.Artifacts = append(plan.Artifacts, artifact{a, rg.Targets[i], CurrentFingerprint(rg.Targets[i])})
	}
	out, _ := json.MarshalIndent(plan, "", "  ")
	return string(out) + "\n"
}

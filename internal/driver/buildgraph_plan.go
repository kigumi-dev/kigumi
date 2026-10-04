package driver

import (
	"encoding/json"

	"kigumi/internal/buildgraph"
	"kigumi/internal/version"
)

// PlanJSON is `kigumi build --plan`: the resolved graph as data, before
// any tool runs or artifact compiles.
func PlanJSON(rg *ResolvedGraph) string {
	type artifact struct {
		*buildgraph.Artifact
		Resolved Target `json:"resolved"`
	}
	// resolvedFramework adds the winning descriptor's directory to the ref.
	type resolvedFramework struct {
		buildgraph.FrameworkRef
		Provider string `json:"provider,omitempty"`
	}
	plan := struct {
		FormatVersion int                    `json:"formatVersion"`
		Toolchain     version.Provenance     `json:"toolchain"`
		Request       buildgraph.Request     `json:"request"`
		Frameworks    []resolvedFramework    `json:"frameworks"`
		Secrets       []buildgraph.SecretRef `json:"secrets"`
		Files         []buildgraph.FileRef   `json:"files"`
		ToolSteps     []buildgraph.ToolStep  `json:"toolSteps"`
		Artifacts     []artifact             `json:"artifacts"`
		Order         []string               `json:"order"`
	}{FormatVersion: version.BuildPlanFormatVersion, Toolchain: version.Current(), Request: rg.Graph.Request, Secrets: rg.Graph.Secrets, Files: rg.Graph.Files, ToolSteps: rg.Graph.ToolSteps, Artifacts: []artifact{}, Order: rg.Order}
	for i, f := range rg.Graph.Frameworks {
		provider := ""
		if i < len(rg.Frameworks) {
			provider = rg.Frameworks[i].Dir
		}
		plan.Frameworks = append(plan.Frameworks, resolvedFramework{FrameworkRef: f, Provider: provider})
	}
	for i, a := range rg.Graph.Artifacts {
		plan.Artifacts = append(plan.Artifacts, artifact{a, rg.Targets[i]})
	}
	out, _ := json.MarshalIndent(plan, "", "  ")
	return string(out) + "\n"
}

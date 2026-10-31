package driver

import (
	"encoding/json"

	"kigumi/internal/toolchain/buildgraph"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/version"
)

// PlanJSON is `kigumi build --plan`: the resolved graph as data, before any tool runs.
func PlanJSON(rg *ResolvedGraph, p *BuildProgram) string {
	out, _ := PlanJSONWithStats(rg, p)
	return out
}

// PlanJSONWithStats is PlanJSON plus how many artifact analyses the plan ran or reused.
func PlanJSONWithStats(rg *ResolvedGraph, p *BuildProgram) (string, PlanStats) {
	type artifact struct {
		*buildgraph.Artifact
		Resolved         Target                     `json:"resolved"`
		Fingerprint      ArtifactFingerprint        `json:"fingerprint"`
		DeriveProvenance []schema.GeneratedArtifact `json:"deriveProvenance,omitempty"`
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
	}{FormatVersion: version.BuildPlanFormatVersion, Toolchain: version.Current(), NativeToolchain: rg.options.CCompiler(), Request: rg.Graph.Request, Secrets: rg.Graph.Secrets, Files: rg.Graph.Files, ToolSteps: rg.Graph.ToolSteps, Artifacts: []artifact{}, Order: rg.Order}
	zigVersion := ""
	cache := newPlanCache(rg.options)
	if p != nil {
		plan.DeriveProvenance = schema.BuildProvenance(p.Res, nil)
		if cc, err := CCompilerFor(p.Module); err == nil {
			plan.NativeToolchain, zigVersion = cc, ZigVersionOf(cc)
		}
	}
	for i, f := range rg.Graph.Frameworks {
		provider := ""
		if i < len(rg.Frameworks) {
			provider = rg.Frameworks[i].Dir
		}
		plan.Frameworks = append(plan.Frameworks, resolvedFramework{FrameworkRef: f, Provider: provider})
	}
	for i, a := range rg.Graph.Artifacts {
		var facts *planFacts
		if p != nil {
			facts = cache.artifactFacts(p, a, rg.Targets[i])
		}
		fp := CurrentFingerprint(rg.Targets[i], zigVersion)
		fp.CapabilityProvision, fp.CapabilityReasons = cache.provision(rg.Targets[i]), facts.reasons(rg.Targets[i])
		var prov []schema.GeneratedArtifact
		if facts != nil {
			prov = facts.provenance
		}
		plan.Artifacts = append(plan.Artifacts, artifact{a, rg.Targets[i], fp, prov})
	}
	out, _ := json.MarshalIndent(plan, "", "  ")
	return string(out) + "\n", cache.finish()
}

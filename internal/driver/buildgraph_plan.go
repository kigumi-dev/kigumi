package driver

import (
	"encoding/json"

	"kigumi/internal/buildgraph"
	"kigumi/internal/schema"
	"kigumi/internal/version"
)

// PlanJSON is `kigumi build --plan`: the resolved graph as data, before any
// tool runs; each artifact's own derive provenance rides along, checked at its own Target/Deny/Ceiling without lowering it.
func PlanJSON(rg *ResolvedGraph, p *BuildProgram) string {
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
	}{FormatVersion: version.BuildPlanFormatVersion, Toolchain: version.Current(), NativeToolchain: CCompiler(), Request: rg.Graph.Request, Secrets: rg.Graph.Secrets, Files: rg.Graph.Files, ToolSteps: rg.Graph.ToolSteps, Artifacts: []artifact{}, Order: rg.Order}
	zigVersion := ""
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
		var prov []schema.GeneratedArtifact
		if p != nil {
			prov = artifactProvenance(p, a, rg.Targets[i])
		}
		plan.Artifacts = append(plan.Artifacts, artifact{a, rg.Targets[i], CurrentFingerprint(rg.Targets[i], zigVersion), prov})
	}
	out, _ := json.MarshalIndent(plan, "", "  ")
	return string(out) + "\n"
}

// artifactProvenance scopes to a's own entry set like buildArtifact, so a
// sibling cmd/* artifact's derive usage never leaks in; a load or check failure here is the artifact's own build step's to report, so this yields nothing.
func artifactProvenance(p *BuildProgram, a *buildgraph.Artifact, t Target) []schema.GeneratedArtifact {
	m, err := LoadModule(p.Module.Root, LoadOptions{StdRoot: p.stdRoot, Target: t, Deny: a.Deny, Ceiling: a.Ceiling, ArtifactName: a.Name})
	if err != nil {
		return nil
	}
	entry, err := scriptUnder(m, a.Dir)
	if err != nil {
		return nil
	}
	m.Entry = entry
	m.ExplicitEntry = true
	res, err := CheckOnly(m)
	if err != nil {
		return nil
	}
	return schema.BuildProvenance(res, nil)
}

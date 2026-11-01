package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/buildgraph"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/module"
)

// TestResolveGraphRejectsAmbiguousFrameworkProvider checks that two descriptors providing the same Framework name/version fail, listing both providers.
func TestResolveGraphRejectsAmbiguousFrameworkProvider(t *testing.T) {
	t.Parallel()
	g := &buildgraph.Graph{Frameworks: []buildgraph.FrameworkRef{{ID: 0, Name: "acme/sdk", Version: "1.0"}}}
	descriptors := []driver.FrameworkDescriptor{
		{Name: "acme/sdk", Version: "1.0", Dir: "/root", Tools: map[string]driver.ToolProgram{}},
		{Name: "acme/sdk", Version: "1.0", Dir: "/dep", Tools: map[string]driver.ToolProgram{}},
	}
	_, err := driver.ResolveGraph(testCompiler(), g, descriptors)
	if err == nil {
		t.Fatal("two same-identity descriptors must be rejected")
	}
	if !strings.Contains(err.Error(), "/root") || !strings.Contains(err.Error(), "/dep") {
		t.Fatalf("error must name both providers, got: %v", err)
	}
}

// TestResolveGraphAcceptsUniqueFrameworkProvider is the control: one
// descriptor for a name/version resolves normally.
func TestResolveGraphAcceptsUniqueFrameworkProvider(t *testing.T) {
	t.Parallel()
	g := &buildgraph.Graph{Frameworks: []buildgraph.FrameworkRef{{ID: 0, Name: "acme/sdk", Version: "1.0"}}}
	descriptors := []driver.FrameworkDescriptor{
		{Name: "acme/sdk", Version: "1.0", Dir: "/root", Tools: map[string]driver.ToolProgram{}},
	}
	rg, err := driver.ResolveGraph(testCompiler(), g, descriptors)
	if err != nil {
		t.Fatal(err)
	}
	if len(rg.Frameworks) != 1 || rg.Frameworks[0].Dir != "/root" {
		t.Fatalf("unexpected resolved frameworks: %+v", rg.Frameworks)
	}
	plan := driver.PlanJSON(rg, nil)
	if !strings.Contains(plan, "\"provider\": \"/root\"") {
		t.Fatalf("plan must record the winning provider's directory:\n%s", plan)
	}
}

// TestLoadFrameworksRejectsDuplicateFrameworkRecord checks that two Framework records in one kigumi.framework.kg fail instead of the later one winning.
func TestLoadFrameworksRejectsDuplicateFrameworkRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, module.FrameworkFileName), []byte(
		"Framework { name: \"acme/sdk\" version: \"1.0\" }\n"+
			"Framework { name: \"acme/sdk\" version: \"2.0\" }\n"+
			"Tool { name: \"gen\" program: \"tools/gen.sh\" }\n"), 0o644)
	if _, err := driver.LoadFrameworks([]string{dir}); err == nil {
		t.Fatal("a second Framework record must be rejected")
	}
}

// TestLoadFrameworksRejectsDuplicateToolName checks that a repeated Tool name in one descriptor fails instead of the later program replacing it.
func TestLoadFrameworksRejectsDuplicateToolName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, module.FrameworkFileName), []byte(
		"Framework { name: \"acme/sdk\" version: \"1.0\" }\n"+
			"Tool { name: \"gen\" program: \"tools/gen-old.sh\" }\n"+
			"Tool { name: \"gen\" program: \"tools/gen-new.sh\" }\n"), 0o644)
	if _, err := driver.LoadFrameworks([]string{dir}); err == nil {
		t.Fatal("a second Tool record with the same name must be rejected")
	}
}

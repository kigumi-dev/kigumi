package schema_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/driver"
)

func loadModule(t *testing.T, src string) (*driver.Module, *sem.Result) {
	t.Helper()
	root := t.TempDir()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	return m, res
}

const schemaSrc = `
import web from app/web

@web.get("/users/:id")
fn show(@web.param("id") id: Int) -> Int {
    id
}
`

const webPkgSrc = `
@schema
pub type Metadata = {
    tag String
}

pub pure fn get(path: String) -> Metadata {
    Metadata { tag: path }
}

pub pure fn param(name: String) -> Metadata {
    Metadata { tag: name }
}
`

func loadWebModule(t *testing.T) (*driver.Module, *sem.Result) {
	t.Helper()
	root := t.TempDir()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "web"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "web", "web.kg"), []byte(webPkgSrc), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(schemaSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	return m, res
}

func TestBuildMetadataSchema(t *testing.T) {
	m, res := loadWebModule(t)
	mod, _ := schema.Build(m, res, schema.Options{})
	var show *schema.Declaration
	for _, d := range mod.Declarations {
		if d.Name == "show" {
			show = d
		}
	}
	if show == nil {
		t.Fatal("show declaration not found")
	}
	if len(show.Metadata) != 1 || show.Metadata[0].Name != "web.get" || show.Metadata[0].SchemaID == "" {
		t.Fatalf("unexpected metadata: %+v", show.Metadata)
	}
	if len(show.Params) != 1 || len(show.Params[0].Metadata) != 1 || show.Params[0].Metadata[0].Name != "web.param" {
		t.Fatalf("unexpected param metadata: %+v", show.Params)
	}
}

func TestBuildDeterministic(t *testing.T) {
	m1, res1 := loadWebModule(t)
	mod1, _ := schema.Build(m1, res1, schema.Options{})
	out1, err := schema.ProjectJSON(mod1)
	if err != nil {
		t.Fatal(err)
	}
	m2, res2 := loadWebModule(t)
	mod2, _ := schema.Build(m2, res2, schema.Options{})
	out2, err := schema.ProjectJSON(mod2)
	if err != nil {
		t.Fatal(err)
	}
	if string(out1) != string(out2) {
		t.Fatalf("non-deterministic projection:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
}

func TestProjectJSONEnvelope(t *testing.T) {
	m, res := loadWebModule(t)
	mod, _ := schema.Build(m, res, schema.Options{})
	out, err := schema.ProjectJSON(mod)
	if err != nil {
		t.Fatal(err)
	}
	env, err := schema.DecodeEnvelope(out)
	if err != nil {
		t.Fatal(err)
	}
	if env.Major == 0 {
		t.Fatalf("envelope missing major version: %+v", env)
	}
	if len(env.Scope) == 0 || env.Scope[0] != "app" && env.Scope[0] != "" {
		t.Fatalf("unexpected scope: %+v", env.Scope)
	}
}

func TestDecodeEnvelopeRejectsUnknownMajor(t *testing.T) {
	bad := []byte(`{"format":"kigumi.schema","major":999,"minor":0}`)
	if _, err := schema.DecodeEnvelope(bad); err == nil {
		t.Fatal("expected an unknown major version to be rejected")
	}
}

func TestDecodeEnvelopeRejectsWrongFormat(t *testing.T) {
	bad := []byte(`{"format":"something.else","major":1,"minor":0}`)
	if _, err := schema.DecodeEnvelope(bad); err == nil {
		t.Fatal("expected a mismatched format name to be rejected")
	}
}

// loadTwoPackageModule declares the same function name in two sibling
// packages, reproducing the ID collision from hashing before Package is set.
func loadTwoPackageModule(t *testing.T) (*driver.Module, *sem.Result) {
	t.Helper()
	root := t.TempDir()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(root, "a"), 0o755)
	os.MkdirAll(filepath.Join(root, "b"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a", "a.kg"), []byte("pub pure fn widget() -> Int {\n    1\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b", "b.kg"), []byte("pub pure fn widget() -> Int {\n    2\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import a from app/a\nimport b from app/b\n\nprint \"${a.widget()} ${b.widget()}\"\n"), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	return m, res
}

func TestBuildDeclarationIDIncludesPackage(t *testing.T) {
	m, res := loadTwoPackageModule(t)
	mod, _ := schema.Build(m, res, schema.Options{})
	ids := map[string]string{}
	for _, d := range mod.Declarations {
		if d.Name != "widget" {
			continue
		}
		if prev, ok := ids[d.ID]; ok {
			t.Fatalf("packages %q and %q both produced declaration ID %q", prev, d.Package, d.ID)
		}
		ids[d.ID] = d.Package
		if d.Package == "" {
			t.Fatalf("declaration %+v has an empty Package", d)
		}
		if want := d.Package + "#fn:widget"; d.ID != want {
			t.Fatalf("id = %q, want %q", d.ID, want)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("expected two distinct widget declarations, got %v", ids)
	}
}

func TestBuildEnvelopeTargetProfile(t *testing.T) {
	m, res := loadWebModule(t)
	mod, _ := schema.Build(m, res, schema.Options{Target: "x86_64-linux", Profile: "recoverable-only"})
	if mod.Envelope.Target != "x86_64-linux" || mod.Envelope.Profile != "recoverable-only" {
		t.Fatalf("envelope did not carry Target/Profile: %+v", mod.Envelope)
	}
	out, err := schema.ProjectJSON(mod)
	if err != nil {
		t.Fatal(err)
	}
	env, err := schema.DecodeEnvelope(out)
	if err != nil {
		t.Fatal(err)
	}
	if env.Target != "x86_64-linux" || env.Profile != "recoverable-only" {
		t.Fatalf("decoded envelope lost Target/Profile: %+v", env)
	}
}

func TestBuildAvailability(t *testing.T) {
	root := t.TempDir()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main_hosted.kg"), []byte("pub pure fn tagged() -> Int {\n    1\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("pub pure fn untagged() -> Int {\n    1\n}\n\nprint \"${tagged()} ${untagged()}\"\n"), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	mod, _ := schema.Build(m, res, schema.Options{})
	var tagged, untagged *schema.Declaration
	for _, d := range mod.Declarations {
		switch d.Name {
		case "tagged":
			tagged = d
		case "untagged":
			untagged = d
		}
	}
	if tagged == nil || untagged == nil {
		t.Fatalf("expected both declarations, got %+v", mod.Declarations)
	}
	if tagged.Availability == nil || tagged.Availability.OS != "hosted" {
		t.Fatalf("tagged.Availability = %+v, want OS \"hosted\"", tagged.Availability)
	}
	if untagged.Availability != nil {
		t.Fatalf("untagged.Availability = %+v, want nil", untagged.Availability)
	}
}

func TestAttrFilter(t *testing.T) {
	m, res := loadWebModule(t)
	mod, _ := schema.Build(m, res, schema.Options{Attr: "web.get"})
	for _, d := range mod.Declarations {
		if d.Name != "show" {
			t.Errorf("--attr should filter out %q", d.Name)
		}
	}
	if len(mod.Declarations) != 1 {
		t.Fatalf("expected exactly show, got %d declarations", len(mod.Declarations))
	}
}

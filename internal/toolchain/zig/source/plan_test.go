package source_test

import (
	"reflect"
	"slices"
	"testing"

	"kigumi/internal/toolchain/zig/source"
)

func TestPlanLocalCheckoutNoRef(t *testing.T) {
	steps, err := source.Plan(source.Request{Checkout: "/src/zig", StageDir: "/store/0.17.0", Jobs: 4})
	if err != nil {
		t.Fatal(err)
	}
	want := []source.Step{
		{Argv: []string{"cmake", "-S", "/src/zig", "-B", "/src/zig/build", "-DCMAKE_INSTALL_PREFIX=/store/0.17.0", "-DCMAKE_BUILD_TYPE=Release"}},
		{Argv: []string{"cmake", "--build", "/src/zig/build", "--target", "install", "-j", "4"}},
	}
	if !reflect.DeepEqual(steps, want) {
		t.Fatalf("got %#v, want %#v", steps, want)
	}
}

func TestPlanClonesWhenURLGiven(t *testing.T) {
	steps, err := source.Plan(source.Request{URL: "https://example/zig.git", Ref: "v0.17.0", Checkout: "/tmp/co", StageDir: "/store/0.17.0", Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 4 {
		t.Fatalf("want clone, checkout, cmake configure, cmake build: got %#v", steps)
	}
	if steps[0].Argv[0] != "git" || steps[0].Argv[1] != "clone" {
		t.Fatalf("step 0 = %#v", steps[0])
	}
	if !slices.Contains(steps[0].Argv, "--") {
		t.Fatalf("clone must separate options from the URL with \"--\": %#v", steps[0])
	}
	if steps[1].Dir != "/tmp/co" || steps[1].Argv[len(steps[1].Argv)-1] != "v0.17.0" {
		t.Fatalf("step 1 = %#v", steps[1])
	}
}

func TestPlanChecksOutRefWithoutURL(t *testing.T) {
	steps, err := source.Plan(source.Request{Ref: "deadbeef", Checkout: "/tmp/co", StageDir: "/store/x", Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || steps[0].Argv[0] != "git" {
		t.Fatalf("got %#v", steps)
	}
}

func TestPlanRejectsDashPrefixedURLOrRef(t *testing.T) {
	if _, err := source.Plan(source.Request{URL: "--upload-pack=touch x", Checkout: "/tmp/co", StageDir: "/store/x"}); err == nil {
		t.Fatal("expected an error for a URL starting with \"-\"")
	}
	if _, err := source.Plan(source.Request{Ref: "--upload-pack=touch x", Checkout: "/tmp/co", StageDir: "/store/x"}); err == nil {
		t.Fatal("expected an error for a Ref starting with \"-\"")
	}
}

func TestPlanRequiresCheckoutAndStageDir(t *testing.T) {
	if _, err := source.Plan(source.Request{StageDir: "/store/x"}); err == nil {
		t.Fatal("expected an error with no Checkout")
	}
	if _, err := source.Plan(source.Request{Checkout: "/tmp/co"}); err == nil {
		t.Fatal("expected an error with no StageDir")
	}
}

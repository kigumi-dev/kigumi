package buildgraph_test

import (
	"encoding/json"
	"testing"

	"kigumi/internal/toolchain/buildgraph"
	buildconfig "kigumi/internal/toolchain/config"
)

// graphTarget is the projection the driver performs when it records a
// resolved target: the graph keeps the triple, sys and allocator, and an
// empty triple means the host whose OS/Arch the resolver supplies again.
func graphTarget(t buildconfig.Target) buildgraph.Target {
	return buildgraph.Target{Triple: t.Triple, Sys: t.Sys, Allocator: t.Allocator}
}

// TestGraphTargetRoundTrip is the graph's half of the target quadruple: a graph
// records only Triple/Sys/Allocator, so resolving it with the same host must
// give back exactly the target it was projected from, and projecting that
// again must give the same graph target.
func TestGraphTargetRoundTrip(t *testing.T) {
	t.Parallel()
	host := buildconfig.HostTarget()
	options := buildconfig.CompilerOptions{Host: host}
	cases := []buildconfig.Target{host}
	for _, c := range []struct{ triple, sys string }{
		{"x86_64-linux", ""},
		{"aarch64-linux-gnu", ""},
		{"x86_64-windows-msvc", ""},
		{"wasm32-wasi", ""},
		{"x86_64-freestanding", ""},
		{"aarch64-uefi", ""},
		{"x86_64-linux", "none"},
	} {
		target, err := buildconfig.ParseTarget(options, c.triple, c.sys)
		if err != nil {
			t.Fatalf("%s/%s: %v", c.triple, c.sys, err)
		}
		cases = append(cases, target)
	}
	for _, target := range cases {
		gt := graphTarget(target)
		back, err := buildconfig.ParseTarget(options, gt.Triple, gt.Sys)
		if err != nil {
			t.Fatalf("%+v: resolving %+v: %v", target, gt, err)
		}
		back.Allocator = gt.Allocator
		if back != target {
			t.Errorf("round trip drifted: target %+v -> %+v -> %+v", target, gt, back)
		}
		if again := graphTarget(back); again != gt {
			t.Errorf("re-projection drifted: %+v -> %+v", gt, again)
		}
	}
	policy := host
	policy.Allocator = "none"
	if got := graphTarget(policy); got.Allocator != "none" || got.Triple != "" || got.Sys != "posix" {
		t.Errorf("the graph projection lost the allocator: %+v", got)
	}
}

// TestTargetJSONShape pins the public graph JSON: the projection adds no
// fields, so an existing plan keeps the same encoding.
func TestTargetJSONShape(t *testing.T) {
	t.Parallel()
	cases := []struct {
		target buildconfig.Target
		want   string
	}{
		{buildconfig.HostTarget(), `{"sys":"posix"}`},
		{buildconfig.Target{}, `{}`},
		{buildconfig.Target{Triple: "x86_64-linux", Sys: "posix"}, `{"triple":"x86_64-linux","sys":"posix"}`},
		{buildconfig.Target{Triple: "x86_64-freestanding", Sys: "none", Allocator: "none"}, `{"triple":"x86_64-freestanding","sys":"none","allocator":"none"}`},
	}
	for _, c := range cases {
		data, err := json.Marshal(graphTarget(c.target))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != c.want {
			t.Errorf("TargetOf(%+v) JSON = %s, want %s", c.target, data, c.want)
		}
	}
}

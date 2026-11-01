package vcs

import (
	"os/exec"
	"strings"
	"testing"

	"kigumi/internal/toolchain/module"
)

func TestGitRefusesUnsetLookOrEnv(t *testing.T) {
	r := module.Require{Name: "example.com/x", Version: "v1.0.0", URL: "https://example.com/x.git"}
	for i, g := range []Git{{}, {Look: exec.LookPath}, {Env: []string{}}} {
		if _, err := g.LatestTag(r); err == nil || !strings.Contains(err.Error(), "needs Look and Env") {
			t.Errorf("case %d: want a refusal naming Look and Env, got %v", i, err)
		}
	}
}

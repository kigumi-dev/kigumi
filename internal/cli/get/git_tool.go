package get

import (
	"os"
	"os/exec"
	"slices"
	"strings"

	"kigumi/internal/cli/shared"
	"kigumi/internal/errors"
)

const allowedGitProtocols = "https:ssh:git:file"

func gitCommand(args ...string) (*exec.Cmd, error) {
	path, err := shared.CompilerOptions().LookTool("git")
	if err != nil {
		return nil, errors.WrapErr(err, "find git")
	}
	g := exec.Command(path, args...)
	user, set := os.LookupEnv("GIT_ALLOW_PROTOCOL")
	g.Env = append(os.Environ(), "GIT_ALLOW_PROTOCOL="+gitProtocols(user, set))
	return g, nil
}

func gitProtocols(user string, set bool) string {
	if !set {
		return allowedGitProtocols
	}
	var keep []string
	for _, p := range strings.Split(user, ":") {
		if slices.Contains(strings.Split(allowedGitProtocols, ":"), p) {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, ":")
}

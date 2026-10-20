package get

import (
	"os/exec"

	"kigumi/internal/cli/shared"
	"kigumi/internal/errors"
)

func gitCommand(args ...string) (*exec.Cmd, error) {
	path, err := shared.CompilerOptions().LookTool("git")
	if err != nil {
		return nil, errors.WrapErr(err, "find git")
	}
	return exec.Command(path, args...), nil
}

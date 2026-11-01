package get

import (
	"os"

	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module/vcs"
)

// newGit hands the module fetcher git from PATH, with the process environment narrowed to the allowed transport protocols.
func newGit(io session.IO) vcs.Git {
	user, set := os.LookupEnv("GIT_ALLOW_PROTOCOL")
	return vcs.Git{
		Look:   session.CompilerOptions().LookTool,
		Env:    append(os.Environ(), "GIT_ALLOW_PROTOCOL="+vcs.Protocols(user, set)),
		Stderr: io.Err,
		Log:    io.Msg,
	}
}

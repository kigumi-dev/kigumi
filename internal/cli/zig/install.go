package zig

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/errors"
	"kigumi/internal/sem"
	"kigumi/internal/version"
	"kigumi/internal/zigtool/fetch"
	"kigumi/internal/zigtool/source"
	"kigumi/internal/zigtool/store"
)

func installCmd() *cobra.Command {
	var sourceArg, baseURL string
	cmd := &cobra.Command{
		Use:   "install [version]",
		Short: "Download (or, with --source, build) a zig version into the managed store",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := storeRoot(cmd)
			if err != nil {
				return err
			}
			want := defaultVersion()
			if len(args) == 1 {
				want = args[0]
			}
			if sourceArg != "" {
				return installFromSource(cmd, root, want, sourceArg)
			}
			if _, ok := store.Installed(root, want); ok && baseURL == "" {
				cmd.Printf("zig %s already installed\n", want)
				return nil
			}
			m, err := fetch.InstallRelease(fetch.Options{IndexURL: baseURL, StoreRoot: root, Version: want, Stderr: cmd.ErrOrStderr(), Lookup: shared.CompilerOptions().ToolLookup()})
			if err != nil {
				if errors.Is(err, fetch.ErrNoPrebuilt) {
					return errors.WrapErrf(err, "no official tarball for %s; build it instead with --source <git url or local checkout>", want)
				}
				return errors.WrapErrf(err, "install zig %s", want)
			}
			cmd.Printf("zig %s installed (%s %s)\n", m.Version, m.OS, m.Arch)
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceArg, "source", "", "build from a git url or local checkout, optionally @ref, instead of downloading a release")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "override ziglang.org/download/index.json (a mirror, or a test server)")
	return cmd
}

// defaultVersion is what `kigumi zig install` with no argument installs:
// the working directory's module pin, else the toolchain's own ZIG_VERSION pin.
func defaultVersion() string {
	if pin := modulePin(); pin != "" {
		return pin
	}
	return version.Zig
}

func installFromSource(cmd *cobra.Command, root, want, sourceArg string) error {
	url, ref := splitSourceRef(sourceArg)
	checkout := url
	cleanup := func() {}
	if looksLikeURL(url) {
		tmp, err := os.MkdirTemp("", "kigumi-zig-source-")
		if err != nil {
			return err
		}
		checkout = tmp
		cleanup = func() { os.RemoveAll(tmp) }
	}
	defer cleanup()
	req := source.Request{Checkout: checkout, Want: want, Ref: ref, Lookup: shared.CompilerOptions().ToolLookup()}
	if looksLikeURL(url) {
		req.URL = url
	}
	m, err := source.Build(req, root, func(s string) { cmd.PrintErrln(s) })
	var mismatch *source.VersionMismatch
	if errors.As(err, &mismatch) {
		msg, help := sem.FillCode(sem.ZigSourceBuildUnverified, checkout, mismatch.Got, want)
		return errors.NewErrf("%s: %s: %s (%s)", sem.ZigSourceBuildUnverified.Num, sem.ZigSourceBuildUnverified.Name, msg, help)
	}
	if err != nil {
		return errors.WrapErrf(err, "build zig %s from source", want)
	}
	cmd.Printf("zig %s built from source (commit %s%s)\n", m.Version, short(m.Commit), dirtySuffix(m.Dirty))
	return nil
}

func short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func dirtySuffix(dirty bool) string {
	if dirty {
		return ", dirty"
	}
	return ""
}

// splitSourceRef splits "<source>@<ref>" on the last "@"; refs can't
// contain "/" or ":", so an ssh "git@host:path" source's "@" is left alone.
func splitSourceRef(arg string) (src, ref string) {
	i := strings.LastIndex(arg, "@")
	if i < 0 {
		return arg, ""
	}
	after := arg[i+1:]
	if strings.ContainsAny(after, "/:") {
		return arg, ""
	}
	return arg[:i], after
}

func looksLikeURL(s string) bool {
	if _, err := os.Stat(s); err == nil {
		return false
	}
	return strings.Contains(s, "://") || strings.Contains(s, "@") || strings.HasSuffix(s, ".git")
}

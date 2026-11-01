package zig

import (
	"fmt"
	"os"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/session"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/version"
	"kigumi/internal/toolchain/zig/fetch"
	"kigumi/internal/toolchain/zig/source"
	"kigumi/internal/toolchain/zig/store"
)

func runInstall(io session.IO, opts options) error {
	root, err := storeRoot(opts.zigHome)
	if err != nil {
		return err
	}
	want := defaultVersion()
	if len(opts.args) == 1 {
		want = opts.args[0]
	}
	if opts.source != "" {
		return installFromSource(io, root, want, opts.source)
	}
	if _, ok := store.Installed(root, want); ok && opts.baseURL == "" {
		fmt.Fprintf(io.Err, "zig %s already installed\n", want)
		return nil
	}
	m, err := fetch.InstallRelease(fetch.Options{IndexURL: opts.baseURL, StoreRoot: root, Version: want, Stderr: io.Err, Lookup: session.CompilerOptions().ToolLookup()})
	if err != nil {
		if errors.Is(err, fetch.ErrNoPrebuilt) {
			return errors.WrapErrf(err, "no official tarball for %s; build it instead with --source <git url or local checkout>", want)
		}
		return errors.WrapErrf(err, "install zig %s", want)
	}
	fmt.Fprintf(io.Err, "zig %s installed (%s %s)\n", m.Version, m.OS, m.Arch)
	return nil
}

// defaultVersion is what `kigumi zig install` installs with no argument: the module pin, else the toolchain's own.
func defaultVersion() string {
	if pin := modulePin(); pin != "" {
		return pin
	}
	return version.Zig
}

func installFromSource(io session.IO, root, want, sourceArg string) error {
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
	req := source.Request{Checkout: checkout, Want: want, Ref: ref, Lookup: session.CompilerOptions().ToolLookup()}
	if looksLikeURL(url) {
		req.URL = url
	}
	m, err := source.Build(req, root, func(s string) { fmt.Fprintln(io.Err, s) })
	var mismatch *source.VersionMismatch
	if errors.As(err, &mismatch) {
		msg, help := sem.FillCode(sem.ZigSourceBuildUnverified, checkout, mismatch.Got, want)
		return errors.NewErrf("%s: %s: %s (%s)", sem.ZigSourceBuildUnverified.Num, sem.ZigSourceBuildUnverified.Name, msg, help)
	}
	if err != nil {
		return errors.WrapErrf(err, "build zig %s from source", want)
	}
	fmt.Fprintf(io.Err, "zig %s built from source (commit %s%s)\n", m.Version, short(m.Commit), dirtySuffix(m.Dirty))
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

// splitSourceRef splits "<source>@<ref>" on the last "@"; refs hold no "/" or ":", so "git@host:path" stays whole.
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

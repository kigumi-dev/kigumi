package mod

import (
	"encoding/json"
	"fmt"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/module"
	"kigumi/internal/toolchain/version"
)

type manifestJSON struct {
	FormatVersion int                   `json:"formatVersion"`
	Toolchain     version.Provenance    `json:"toolchain"`
	Name          string                `json:"name"`
	Kigumi        string                `json:"kigumi"`
	Zig           string                `json:"zig,omitempty"`
	Requires      []requireJSON         `json:"requires"`
	Links         []module.Link         `json:"links"`
	Sources       []module.NativeSource `json:"sources"`
	Local         map[string]string     `json:"local"`
	Lock          map[string]lockJSON   `json:"lock"`
}

type requireJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url,omitempty"`
}

type lockJSON struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Hash    string `json:"hash"`
}

// runJSON prints mod.kg and mod.lock.kg as one JSON document for tools that do not parse Kigumi.
func runJSON(io session.IO, opts options) error {
	mf, ok, err := module.ReadModFile(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read manifest")
	}
	if !ok {
		return &exit.UsageError{Err: errors.NewErrf("%s has no %s", opts.root, module.ManifestName)}
	}
	lock, err := module.ReadLock(opts.root)
	if err != nil {
		return errors.WrapErr(err, "read lock")
	}
	local, err := module.ReadLocal(session.CompilerOptions(), opts.root)
	if err != nil {
		return errors.WrapErr(err, "read local overrides")
	}
	out := manifestJSON{FormatVersion: version.ModJSONFormatVersion, Toolchain: version.Current(), Name: mf.Name, Kigumi: mf.Kigumi, Zig: mf.Zig, Requires: []requireJSON{}, Links: mf.Links, Sources: mf.Sources, Local: local.Replaces, Lock: map[string]lockJSON{}}
	for _, r := range mf.Requires {
		out.Requires = append(out.Requires, requireJSON{Name: r.Name, Version: r.Version, URL: r.URL})
	}
	for name, e := range lock.Entries {
		out.Lock[name] = lockJSON{Version: e.Version, Commit: e.Commit, Hash: e.Hash}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(io.Out, string(data))
	return nil
}

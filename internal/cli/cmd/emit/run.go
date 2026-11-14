package emit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/version"
)

func runSchema(io session.IO, opts schemaOptions) error {
	m, res, err := load(io, opts.session, opts.target, opts.allowErrors)
	if err != nil {
		return err
	}
	mod, err := build(m, res, opts.attr, opts.session)
	if err != nil {
		return err
	}
	data, err := schema.ProjectDocumentJSON(mod)
	if err != nil {
		return errors.WrapErr(err, "encode schema document")
	}
	return write(io, opts.out, data)
}

func runHeader(io session.IO, opts headerOptions) error {
	m, res, err := load(io, opts.session, opts.target, false)
	if err != nil {
		return err
	}
	mod, err := build(m, res, "", opts.session)
	if err != nil {
		return err
	}
	header := schema.EmitCHeader(mod, schema.CHeaderOptions{Guard: opts.guard})
	if opts.format == "json" {
		data, err := json.MarshalIndent(version.Wrap("emit-c-header", version.CHeaderFormatVersion, header), "", "  ")
		if err != nil {
			return errors.WrapErr(err, "encode C header report")
		}
		return write(io, opts.out, data)
	}
	return write(io, opts.out, []byte(header.Text))
}

// load reads target's whole module and checks it; errOut receives the
// rendered diagnostics, which end the command with 1 unless allowErrors.
func load(io session.IO, s session.Session, target string, allowErrors bool) (*driver.Module, *sem.Result, error) {
	m, err := s.LoadWhole(target, true, io.Err)
	if err != nil {
		return nil, nil, err
	}
	res, err := driver.Check(m)
	if err != nil {
		return nil, nil, errors.WrapErr(err, "check module")
	}
	if res.HasErrors() {
		fmt.Fprint(io.Err, res.Render())
		if !allowErrors {
			return nil, nil, exit.Code(1)
		}
	}
	return m, res, nil
}

// build projects the checked module onto the Schema IR, stamped with the
// target the session resolved.
func build(m *driver.Module, res *sem.Result, attr string, s session.Session) (*schema.Module, error) {
	tgt, err := s.Target()
	if err != nil {
		return nil, &exit.ValueError{Err: err}
	}
	target := tgt.Name()
	mod, _ := schema.Build(m, res, schema.Options{Attr: attr, Target: target, Profile: tgt.Profile})
	return mod, nil
}

// write sends data to out or path, creating path's directory; every emitter
// output ends in one newline.
func write(io session.IO, path string, data []byte) error {
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	if path == "" {
		_, err := io.Out.Write(data)
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errors.WrapErrf(err, "create output directory %s", dir)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return errors.WrapErrf(err, "write %s", path)
	}
	return nil
}

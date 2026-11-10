package build

import (
	"fmt"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/cli/session"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/policy"
)

func runProgram(io session.IO, opts options) error {
	s := opts.session
	pol := s.Policy()
	target, err := s.Target()
	if err != nil {
		return &exit.ValueError{Err: err}
	}
	p, ok, err := driver.LoadBuildProgram(opts.root, driver.LoadOptions{StdRoot: s.StdRoot(), NoLocal: s.NoLocal || session.LocalOff(), Offline: s.Offline, Policy: pol, ZigPath: session.ZigPath(), ZigHome: session.ZigHome(), PackageCache: session.PackageCache(), VerifyPasses: session.VerifyPasses(), Compiler: session.CompilerOptions()})
	if err != nil {
		fmt.Fprint(io.Err, err.Error())
		if !strings.HasSuffix(err.Error(), "\n") {
			fmt.Fprintln(io.Err)
		}
		return exit.Code(1)
	}
	if !ok {
		return errors.NewErrf("%s has no %s", opts.root, driver.BuildFileName)
	}
	report := policy.Evaluate(p.Module.Violations, pol, s.ReleaseViolations(true)...)
	fmt.Fprint(io.Err, report.Render())
	if report.HasErrors() {
		return exit.Code(1)
	}
	g, err := p.Run(target, opts.extra, opts.outDir)
	if err != nil {
		fmt.Fprintln(io.Err, driver.BuildFileName+": "+err.Error())
		return exit.Code(1)
	}
	roots, err := p.DescriptorRoots()
	if err != nil {
		return err
	}
	descriptors, err := driver.LoadFrameworks(roots)
	if err != nil {
		return err
	}
	rg, err := driver.ResolveGraph(session.CompilerOptions(), g, descriptors)
	if err != nil {
		fmt.Fprintln(io.Err, driver.BuildFileName+": "+err.Error())
		return exit.Code(1)
	}
	if opts.plan {
		plan, stats := driver.PlanJSONWithStats(rg, p)
		if len(stats.EntryErrors) > 0 {
			for _, msg := range stats.EntryErrors {
				fmt.Fprintln(io.Err, "build: "+msg)
			}
			return exit.Code(1)
		}
		fmt.Fprint(io.Out, plan)
		return nil
	}
	secrets := map[string]string{}
	for _, spec := range opts.secrets {
		name, path, ok := strings.Cut(spec, "=")
		if !ok {
			return &exit.ValueError{Err: errors.NewErrf("--secret takes name=file, not %q", spec)}
		}
		secrets[name] = path
	}
	for _, s := range rg.Graph.Secrets {
		if _, ok := secrets[s.Name]; !ok {
			if file := session.SecretFile(s.Name); file != "" {
				secrets[s.Name] = file
			}
		}
	}
	reports := map[string]driver.BuildReport{}
	exec := driver.ExecOptions{Secrets: secrets, CFlags: session.CFlags(), ToolEnv: session.ToolEnv(), Reproducible: opts.reproducible, Env: session.CCEnv(), Link: opts.shape.link, SizeMin: opts.shape.sizeMin, Reports: reports}
	if err := driver.ExecuteGraph(p, rg, opts.outDir, exec, io.Err); err != nil {
		fmt.Fprintln(io.Err, "build: "+err.Error())
		return exit.Code(1)
	}
	fmt.Fprint(io.Err, buildSummary(s, p.Module, pol, opts.reproducible, driver.ToolCapabilities(rg.Graph), reports).Render())
	return nil
}

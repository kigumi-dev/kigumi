package build

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/cli/shared"
	"kigumi/internal/cliutil"
	"kigumi/internal/driver"
	"kigumi/internal/errors"
)

func runProgram(cmd *cobra.Command, root string, flags []string, plan bool, outDir string, secretSpecs []string, reproducible bool, policy driver.BuildPolicy) error {
	target, err := shared.Target(cmd)
	if err != nil {
		return &cliutil.UsageError{Err: err}
	}
	p, ok, err := driver.LoadBuildProgram(root, driver.LoadOptions{StdRoot: shared.StdRoot(cmd), Policy: policy, ZigPath: shared.ZigPath(), VerifyPasses: shared.VerifyPasses(), Compiler: shared.CompilerOptions()})
	if err != nil {
		cmd.PrintErr(err.Error())
		if !strings.HasSuffix(err.Error(), "\n") {
			cmd.PrintErrln()
		}
		return cliutil.Exit(1)
	}
	if !ok {
		return errors.NewErrf("%s has no %s", root, driver.BuildFileName)
	}
	report := driver.EvaluateRelease(p.Module, policy, shared.ReleaseViolations(cmd, true)...)
	fmt.Fprint(cmd.ErrOrStderr(), report.Render())
	if report.HasErrors() {
		return cliutil.Exit(1)
	}
	g, err := p.Run(target, flags, outDir)
	if err != nil {
		cmd.PrintErrln(driver.BuildFileName + ": " + err.Error())
		return cliutil.Exit(1)
	}
	roots, err := p.DescriptorRoots()
	if err != nil {
		return err
	}
	descriptors, err := driver.LoadFrameworks(roots)
	if err != nil {
		return err
	}
	rg, err := driver.ResolveGraph(shared.CompilerOptions(), g, descriptors)
	if err != nil {
		cmd.PrintErrln(driver.BuildFileName + ": " + err.Error())
		return cliutil.Exit(1)
	}
	if plan {
		fmt.Fprint(cmd.OutOrStdout(), driver.PlanJSON(rg, p))
		return nil
	}
	secrets := map[string]string{}
	for _, spec := range secretSpecs {
		name, path, ok := strings.Cut(spec, "=")
		if !ok {
			return &cliutil.UsageError{Err: errors.NewErrf("--secret takes name=file, not %q", spec)}
		}
		secrets[name] = path
	}
	for _, s := range rg.Graph.Secrets {
		if _, ok := secrets[s.Name]; !ok {
			if file := shared.SecretFile(s.Name); file != "" {
				secrets[s.Name] = file
			}
		}
	}
	if err := driver.ExecuteGraph(p, rg, outDir, driver.ExecOptions{Secrets: secrets, CFlags: shared.CFlags(), ToolEnv: shared.ToolEnv(), Reproducible: reproducible, Env: shared.CCEnv()}, cmd.ErrOrStderr()); err != nil {
		cmd.PrintErrln("build: " + err.Error())
		return cliutil.Exit(1)
	}
	fmt.Fprint(cmd.ErrOrStderr(), buildSummary(cmd, p.Module, policy, reproducible, driver.ToolCapabilities(rg.Graph)).Render())
	return nil
}

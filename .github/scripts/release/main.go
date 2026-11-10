// Command release builds and verifies the Kigumi release artifacts: the
// deterministic source/header archives, the checksum manifest and the SPDX
// SBOM. The GitHub release workflow calls it, and the same commands run
// locally so a release can be rehearsed without CI.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "version":
		err = runVersion(args)
	case "targets":
		err = runTargets(args)
	case "archive":
		err = runArchive(args)
	case "sbom":
		err = runSBOM(args)
	case "provenance":
		err = runProvenance(args)
	case "generate":
		err = runGenerate(args)
	case "verify":
		err = runVerify(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "release: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `release: Kigumi release artifact tool

usage:
  release version    --root <repo> [--field release|tag|abi|canonical-abi|go] [--expect <tag>] [--json]
  release targets    --file targets.txt [--json]
  release archive    --root <dir> --out <file> [--prefix <dir>] (--git | --all | <paths...>)
  release sbom       --out <file> --version <v> [--commit <sha>] [--repository <owner/repo>] [--created <rfc3339>] --binary <path>...
  release provenance --out <file> --repository <owner/repo> --ref <ref> --commit <sha> --workflow <ref> [--run-id <id>] [--run-attempt <n>] --version <v> --targets-file <file>
  release generate   --dir <dist> --root <repo> --version <v> --tag <v> --commit <sha> --go-version <go> --targets-file <file> [--repository <owner/repo>]
  release verify     --dir <dist> --root <repo> --version <v> --tag <v> --commit <sha> --targets-file <file>
`)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	return nil
}

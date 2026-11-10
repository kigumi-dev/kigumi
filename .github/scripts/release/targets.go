package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Target is one explicit host build target. The release workflow turns this
// list into its build matrix, so the workflows never duplicate the target set.
type Target struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

func (t Target) String() string { return t.GOOS + "/" + t.GOARCH }

// AssetSuffix is the target part of an archive name, e.g. linux_amd64.
func (t Target) AssetSuffix() string { return t.GOOS + "_" + t.GOARCH }

// BinaryName is the installed binary name inside the archive.
func (t Target) BinaryName() string {
	if t.GOOS == "windows" {
		return "kigumi.exe"
	}
	return "kigumi"
}

// readTargets parses the target list: comments and blank lines are ignored.
func readTargets(path string) ([]Target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Target
	seen := map[string]bool{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s:%d: want <os>/<arch>, got %q", path, i+1, line)
		}
		goos, goarch := parts[0], parts[1]
		if goos == "" || goarch == "" || strings.ContainsAny(goos+goarch, " \t") {
			return nil, fmt.Errorf("%s:%d: want <os>/<arch>, got %q", path, i+1, line)
		}
		key := goos + "/" + goarch
		if seen[key] {
			return nil, fmt.Errorf("%s:%d: duplicate target %s", path, i+1, key)
		}
		seen[key] = true
		out = append(out, Target{GOOS: goos, GOARCH: goarch})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no targets", path)
	}
	if !sort.SliceIsSorted(out, func(i, j int) bool { return out[i].String() < out[j].String() }) {
		return nil, fmt.Errorf("%s: targets must be sorted by os/arch", path)
	}
	return out, nil
}

func targetNames(targets []Target) []string {
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.String()
	}
	return names
}

func runTargets(args []string) error {
	fs := newFlagSet("targets")
	file := fs.String("file", "targets.txt", "target list file")
	asJSON := fs.Bool("json", false, "print a GitHub Actions matrix as JSON")
	if err := parse(fs, args); err != nil {
		return err
	}
	targets, err := readTargets(*file)
	if err != nil {
		return err
	}
	if *asJSON {
		matrix := struct {
			Include []matrixEntry `json:"include"`
		}{}
		for _, t := range targets {
			matrix.Include = append(matrix.Include, matrixEntry{
				GOOS:   t.GOOS,
				GOARCH: t.GOARCH,
				Suffix: t.AssetSuffix(),
				Binary: t.BinaryName(),
			})
		}
		data, err := json.Marshal(matrix)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	for _, t := range targets {
		fmt.Println(t.String())
	}
	return nil
}

type matrixEntry struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
	Suffix string `json:"suffix"`
	Binary string `json:"binary"`
}

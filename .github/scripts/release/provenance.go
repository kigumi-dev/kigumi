package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// provenanceDocument is a SLSA v1 provenance statement. GitHub also records a
// signed attestation for the same subjects; this file travels with the release
// so the mapping from tag to commit and target set is readable offline.
type provenanceDocument struct {
	Type          string              `json:"_type"`
	Subject       []provenanceSubject `json:"subject"`
	PredicateType string              `json:"predicateType"`
	Predicate     provenancePredicate `json:"predicate"`
}

type provenanceSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type provenancePredicate struct {
	BuildDefinition provenanceBuildDefinition `json:"buildDefinition"`
	RunDetails      provenanceRunDetails      `json:"runDetails"`
}

type provenanceBuildDefinition struct {
	BuildType            string                   `json:"buildType"`
	ExternalParameters   provenanceExternalParams `json:"externalParameters"`
	ResolvedDependencies []provenanceDependency   `json:"resolvedDependencies"`
}

type provenanceExternalParams struct {
	Version   string   `json:"version"`
	Tag       string   `json:"tag"`
	Targets   []string `json:"targets"`
	Checksums string   `json:"checksums"`
}

type provenanceDependency struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

type provenanceRunDetails struct {
	Builder  provenanceBuilder  `json:"builder"`
	Metadata provenanceMetadata `json:"metadata"`
}

type provenanceBuilder struct {
	ID string `json:"id"`
}

type provenanceMetadata struct {
	InvocationID string `json:"invocationId,omitempty"`
}

func runProvenance(args []string) error {
	fs := newFlagSet("provenance")
	dir := fs.String("dir", "dist", "directory holding the release assets")
	out := fs.String("out", "", "output provenance JSON (must not be inside dir before scanning)")
	repository := fs.String("repository", "kigumi-org/kigumi", "source repository")
	ref := fs.String("ref", "", "source ref, e.g. refs/tags/v0.1.0")
	commit := fs.String("commit", "", "source commit")
	workflow := fs.String("workflow", "", "workflow ref that built the release")
	runID := fs.String("run-id", "", "workflow run id")
	version := fs.String("version", "", "release version")
	targetsFile := fs.String("targets-file", "targets.txt", "target list file")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *out == "" || *commit == "" || *workflow == "" || *version == "" {
		return fmt.Errorf("provenance: --out, --commit, --workflow and --version are required")
	}
	targets, err := readTargets(*targetsFile)
	if err != nil {
		return err
	}
	tag := "v" + strings.TrimPrefix(*version, "v")
	if *ref == "" {
		*ref = "refs/tags/" + tag
	}
	doc, err := buildProvenance(*dir, *out, provenanceInputs{
		Repository: *repository,
		Ref:        *ref,
		Commit:     *commit,
		Workflow:   *workflow,
		RunID:      *runID,
		Version:    *version,
		Tag:        tag,
		Targets:    targetNames(targets),
	})
	if err != nil {
		return err
	}
	return writeJSON(*out, doc)
}

type provenanceInputs struct {
	Repository string
	Ref        string
	Commit     string
	Workflow   string
	RunID      string
	Version    string
	Tag        string
	Targets    []string
}

func buildProvenance(dir, out string, in provenanceInputs) (provenanceDocument, error) {
	subjects, err := hashAssets(dir, out)
	if err != nil {
		return provenanceDocument{}, err
	}
	if len(subjects) == 0 {
		return provenanceDocument{}, fmt.Errorf("provenance: no assets in %s", dir)
	}
	return provenanceDocument{
		Type:          "https://in-toto.io/Statement/v1",
		Subject:       subjects,
		PredicateType: "https://slsa.dev/provenance/v1",
		Predicate: provenancePredicate{
			BuildDefinition: provenanceBuildDefinition{
				BuildType: "https://github.com/" + in.Repository + "/blob/master/docs/release_v1.md",
				ExternalParameters: provenanceExternalParams{
					Version:   in.Version,
					Tag:       in.Tag,
					Targets:   in.Targets,
					Checksums: "SHA256SUMS",
				},
				ResolvedDependencies: []provenanceDependency{{
					URI:    "git+https://github.com/" + in.Repository + "@" + in.Ref,
					Digest: map[string]string{"gitCommit": in.Commit},
				}},
			},
			RunDetails: provenanceRunDetails{
				Builder:  provenanceBuilder{ID: in.Workflow},
				Metadata: provenanceMetadata{InvocationID: in.RunID},
			},
		},
	}, nil
}

// hashAssets returns every regular file under dir except out itself, sorted by
// name, with its SHA-256. The manifest and checksum file are generated after
// this, so they are not subjects here; GitHub attests them separately.
func hashAssets(dir, out string) ([]provenanceSubject, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var subjects []provenanceSubject
	outPath := filepath.Clean(out)
	for _, e := range entries {
		if e.IsDir() || !e.Type().IsRegular() {
			return nil, fmt.Errorf("provenance: %s is not a regular file", filepath.Join(dir, e.Name()))
		}
		p := filepath.Join(dir, e.Name())
		if filepath.Clean(p) == outPath {
			continue
		}
		sum, err := fileSHA256(p)
		if err != nil {
			return nil, err
		}
		subjects = append(subjects, provenanceSubject{
			Name:   e.Name(),
			Digest: map[string]string{"sha256": sum},
		})
	}
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })
	return subjects, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeJSON(path string, v any) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

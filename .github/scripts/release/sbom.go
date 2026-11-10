package main

import (
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// spdxDocument is the subset of SPDX 2.3 JSON a release needs: one package
// per module linked into the shipped binaries, related to the toolchain
// package by DEPENDS_ON.
type spdxDocument struct {
	SPDXVersion       string             `json:"spdxVersion"`
	DataLicense       string             `json:"dataLicense"`
	SPDXID            string             `json:"SPDXID"`
	Name              string             `json:"name"`
	DocumentNamespace string             `json:"documentNamespace"`
	CreationInfo      spdxCreationInfo   `json:"creationInfo"`
	Packages          []spdxPackage      `json:"packages"`
	Relationships     []spdxRelationship `json:"relationships"`
}

type spdxCreationInfo struct {
	Created  string   `json:"created"`
	Creators []string `json:"creators"`
	Comment  string   `json:"comment,omitempty"`
}

type spdxPackage struct {
	SPDXID           string            `json:"SPDXID"`
	Name             string            `json:"name"`
	VersionInfo      string            `json:"versionInfo"`
	DownloadLocation string            `json:"downloadLocation"`
	FilesAnalyzed    bool              `json:"filesAnalyzed"`
	LicenseConcluded string            `json:"licenseConcluded"`
	LicenseDeclared  string            `json:"licenseDeclared"`
	Supplier         string            `json:"supplier"`
	ExternalRefs     []spdxExternalRef `json:"externalRefs,omitempty"`
}

type spdxExternalRef struct {
	ReferenceCategory string `json:"referenceCategory"`
	ReferenceType     string `json:"referenceType"`
	ReferenceLocator  string `json:"referenceLocator"`
}

type spdxRelationship struct {
	SPDXElementID      string `json:"spdxElementId"`
	RelationshipType   string `json:"relationshipType"`
	RelatedSPDXElement string `json:"relatedSpdxElement"`
}

const toolPackageID = "SPDXRef-Package-kigumi"

// buildFacts is what every shipped binary must agree on: the VCS revision it
// was built from and whether the tree was dirty. A release must never contain
// a binary that disagrees with the tag or carries a dirty marker.
type buildFacts struct {
	Commit     string
	Dirty      bool
	GoVersion  string
	MainModule string
}

func inspectBinary(path string) (buildFacts, *buildinfo.BuildInfo, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return buildFacts{}, nil, fmt.Errorf("read build info from %s: %w", path, err)
	}
	f := buildFacts{GoVersion: info.GoVersion, MainModule: info.Main.Path}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			f.Commit = s.Value
		case "vcs.modified":
			f.Dirty = s.Value == "true"
		}
	}
	return f, info, nil
}

type sbomOptions struct {
	Version    string
	Tag        string
	Commit     string
	Repository string
	Created    string
}

func runSBOM(args []string) error {
	fs := newFlagSet("sbom")
	out := fs.String("out", "", "output SPDX JSON")
	version := fs.String("version", "", "release version, e.g. 0.1.0")
	commit := fs.String("commit", "", "expected source commit")
	repository := fs.String("repository", "kigumi-org/kigumi", "source repository")
	created := fs.String("created", "", "creation timestamp (RFC3339 or Unix seconds); default SOURCE_DATE_EPOCH or now")
	var binaries []string
	fs.Func("binary", "shipped binary (repeatable)", func(v string) error {
		binaries = append(binaries, v)
		return nil
	})
	if err := parse(fs, args); err != nil {
		return err
	}
	if *out == "" || *version == "" {
		return fmt.Errorf("sbom: --out and --version are required")
	}
	if len(binaries) == 0 {
		return fmt.Errorf("sbom: at least one --binary is required")
	}
	doc, err := buildSBOM(sbomOptions{
		Version:    *version,
		Tag:        "v" + strings.TrimPrefix(*version, "v"),
		Commit:     *commit,
		Repository: *repository,
		Created:    *created,
	}, binaries)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(*out, data, 0o644)
}

func buildSBOM(opts sbomOptions, binaries []string) (spdxDocument, error) {
	var infos []*buildinfo.BuildInfo
	for _, bin := range binaries {
		_, info, err := inspectBinary(bin)
		if err != nil {
			return spdxDocument{}, err
		}
		infos = append(infos, info)
	}
	return buildSBOMFromInfos(opts, infos)
}

// buildSBOMFromInfos is the pure form used by tests: the binaries were already
// read, and every one of them must agree on commit and Go version.
func buildSBOMFromInfos(opts sbomOptions, infos []*buildinfo.BuildInfo) (spdxDocument, error) {
	created, err := resolveCreated(opts.Created)
	if err != nil {
		return spdxDocument{}, err
	}
	if len(infos) == 0 {
		return spdxDocument{}, fmt.Errorf("sbom: no binaries")
	}
	type dep struct{ path, version string }
	deps := map[string]dep{}
	var facts buildFacts
	for i, info := range infos {
		f := buildFacts{GoVersion: info.GoVersion, MainModule: info.Main.Path}
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				f.Commit = s.Value
			case "vcs.modified":
				f.Dirty = s.Value == "true"
			}
		}
		if f.Commit == "" {
			return spdxDocument{}, fmt.Errorf("binary %d: build info has no vcs.revision; build with -buildvcs=true", i)
		}
		if f.Dirty {
			return spdxDocument{}, fmt.Errorf("binary %d: binary was built from a dirty tree", i)
		}
		if opts.Commit != "" && f.Commit != opts.Commit {
			return spdxDocument{}, fmt.Errorf("binary %d: built from %s, want %s", i, f.Commit, opts.Commit)
		}
		if i == 0 {
			facts = f
		} else if f.Commit != facts.Commit || f.GoVersion != facts.GoVersion {
			return spdxDocument{}, fmt.Errorf("binary %d: build provenance differs from binary 0", i)
		}
		for _, m := range info.Deps {
			if m.Replace != nil {
				m = m.Replace
			}
			if m.Version == "" || m.Version == "(devel)" {
				continue
			}
			key := m.Path + "@" + m.Version
			deps[key] = dep{path: m.Path, version: m.Version}
		}
	}
	keys := make([]string, 0, len(deps))
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	doc := spdxDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              "kigumi-" + opts.Version,
		DocumentNamespace: fmt.Sprintf("https://github.com/%s/releases/tag/%s/spdx", opts.Repository, opts.Tag),
		CreationInfo: spdxCreationInfo{
			Created:  created,
			Creators: []string{"Tool: kigumi-release"},
			Comment:  fmt.Sprintf("commit=%s;dirty=false;go=%s", facts.Commit, facts.GoVersion),
		},
	}
	doc.Packages = append(doc.Packages, spdxPackage{
		SPDXID:           toolPackageID,
		Name:             "kigumi",
		VersionInfo:      opts.Version,
		DownloadLocation: "NOASSERTION",
		FilesAnalyzed:    false,
		LicenseConcluded: "NOASSERTION",
		LicenseDeclared:  "MIT",
		Supplier:         "NOASSERTION",
		ExternalRefs: []spdxExternalRef{{
			ReferenceCategory: "PACKAGE-MANAGER",
			ReferenceType:     "purl",
			ReferenceLocator:  "pkg:golang/github.com/" + opts.Repository + "@" + opts.Version,
		}},
	})
	doc.Relationships = append(doc.Relationships, spdxRelationship{
		SPDXElementID:      "SPDXRef-DOCUMENT",
		RelationshipType:   "DESCRIBES",
		RelatedSPDXElement: toolPackageID,
	})
	for _, key := range keys {
		d := deps[key]
		id := spdxID(d.path, d.version)
		doc.Packages = append(doc.Packages, spdxPackage{
			SPDXID:           id,
			Name:             d.path,
			VersionInfo:      d.version,
			DownloadLocation: "NOASSERTION",
			FilesAnalyzed:    false,
			LicenseConcluded: "NOASSERTION",
			LicenseDeclared:  "NOASSERTION",
			Supplier:         "NOASSERTION",
			ExternalRefs: []spdxExternalRef{{
				ReferenceCategory: "PACKAGE-MANAGER",
				ReferenceType:     "purl",
				ReferenceLocator:  "pkg:golang/" + d.path + "@" + d.version,
			}},
		})
		doc.Relationships = append(doc.Relationships, spdxRelationship{
			SPDXElementID:      toolPackageID,
			RelationshipType:   "DEPENDS_ON",
			RelatedSPDXElement: id,
		})
	}
	return doc, nil
}

func spdxID(modulePath, version string) string {
	repl := func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}
	return "SPDXRef-Package-" + strings.Map(repl, modulePath+"-"+version)
}

func resolveCreated(v string) (string, error) {
	if v == "" {
		if epoch := os.Getenv("SOURCE_DATE_EPOCH"); epoch != "" {
			v = epoch
		}
	}
	if v == "" {
		return time.Now().UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if secs, err := time.ParseDuration(v + "s"); err == nil {
		return time.Unix(int64(secs/time.Second), 0).UTC().Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("sbom: --created %q is not RFC3339 or Unix seconds", v)
}

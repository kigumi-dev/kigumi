package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	manifestName  = "release-manifest.json"
	checksumsName = "SHA256SUMS"
	sbomName      = "SBOM.spdx.json"
	provName      = "provenance.json"
)

// manifest is the machine-readable index of a release. The digest list covers
// every published file except the manifest and the checksum file itself.
type manifest struct {
	FormatVersion       int             `json:"formatVersion"`
	Kind                string          `json:"kind"`
	Repository          string          `json:"repository"`
	Release             string          `json:"release"`
	Tag                 string          `json:"tag"`
	Commit              string          `json:"commit"`
	GoVersion           string          `json:"goVersion"`
	ABIRevision         int             `json:"abiRevision"`
	CanonicalABIVersion string          `json:"canonicalAbiVersion"`
	Targets             []string        `json:"targets"`
	Assets              []manifestAsset `json:"assets"`
	SBOM                string          `json:"sbom"`
	Provenance          string          `json:"provenance"`
}

type manifestAsset struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func runGenerate(args []string) error {
	fs := newFlagSet("generate")
	dir := fs.String("dir", "dist", "release asset directory")
	root := fs.String("root", ".", "repository root")
	version := fs.String("version", "", "release version (without v)")
	tag := fs.String("tag", "", "release tag")
	commit := fs.String("commit", "", "source commit")
	goVersion := fs.String("go-version", "", "Go version used to build the binaries")
	targetsFile := fs.String("targets-file", "targets.txt", "target list file")
	repository := fs.String("repository", "kigumi-org/kigumi", "source repository")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *version == "" || *tag == "" || *commit == "" || *goVersion == "" {
		return fmt.Errorf("generate: --version, --tag, --commit and --go-version are required")
	}
	targets, err := readTargets(*targetsFile)
	if err != nil {
		return err
	}
	m, err := generateManifest(generateInputs{
		Dir:        *dir,
		Root:       *root,
		Version:    *version,
		Tag:        *tag,
		Commit:     *commit,
		GoVersion:  *goVersion,
		Repository: *repository,
		Targets:    targets,
	})
	if err != nil {
		return err
	}
	return writeManifest(*dir, m)
}

type generateInputs struct {
	Dir        string
	Root       string
	Version    string
	Tag        string
	Commit     string
	GoVersion  string
	Repository string
	Targets    []Target
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
var versionPattern = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]*$`)

func generateManifest(in generateInputs) (manifest, error) {
	vi, err := readVersion(in.Root)
	if err != nil {
		return manifest{}, err
	}
	if in.Version != vi.Release {
		return manifest{}, fmt.Errorf("version %q does not match version.go Release %q", in.Version, vi.Release)
	}
	if in.Tag != vi.Tag {
		return manifest{}, fmt.Errorf("tag %q does not match version.go %q", in.Tag, vi.Tag)
	}
	if !versionPattern.MatchString(in.Version) {
		return manifest{}, fmt.Errorf("version %q is not a safe archive name", in.Version)
	}
	if !commitPattern.MatchString(in.Commit) {
		return manifest{}, fmt.Errorf("commit %q is not a git object id", in.Commit)
	}
	if strings.ContainsAny(in.GoVersion, " \t\n") {
		return manifest{}, fmt.Errorf("go version %q contains whitespace", in.GoVersion)
	}
	entries, err := assetFiles(in.Dir)
	if err != nil {
		return manifest{}, err
	}
	expected, err := expectedAssets(in.Version, in.Targets)
	if err != nil {
		return manifest{}, err
	}
	have := map[string]bool{}
	for _, e := range entries {
		if e.Name() == manifestName || e.Name() == checksumsName {
			continue
		}
		have[e.Name()] = true
	}
	for _, name := range expected {
		if !have[name] {
			return manifest{}, fmt.Errorf("%s: missing release asset %s", in.Dir, name)
		}
		delete(have, name)
	}
	for name := range have {
		return manifest{}, fmt.Errorf("%s: unexpected file %s (targets are %s)", in.Dir, name, strings.Join(targetNames(in.Targets), ", "))
	}
	m := manifest{
		FormatVersion:       1,
		Kind:                "release-manifest",
		Repository:          in.Repository,
		Release:             in.Version,
		Tag:                 in.Tag,
		Commit:              in.Commit,
		GoVersion:           in.GoVersion,
		ABIRevision:         vi.ABIRevision,
		CanonicalABIVersion: vi.CanonicalABIVersion,
		Targets:             targetNames(in.Targets),
		SBOM:                sbomName,
		Provenance:          provName,
	}
	for _, e := range entries {
		if e.Name() == manifestName || e.Name() == checksumsName {
			continue
		}
		sum, size, err := hashFile(filepath.Join(in.Dir, e.Name()))
		if err != nil {
			return manifest{}, err
		}
		if size == 0 {
			return manifest{}, fmt.Errorf("%s: release asset %s is empty", in.Dir, e.Name())
		}
		m.Assets = append(m.Assets, manifestAsset{Name: e.Name(), SHA256: sum, Size: size})
	}
	sort.Slice(m.Assets, func(i, j int) bool { return m.Assets[i].Name < m.Assets[j].Name })
	if err := validateSBOM(filepath.Join(in.Dir, sbomName), m); err != nil {
		return manifest{}, err
	}
	if err := validateProvenance(filepath.Join(in.Dir, provName), m); err != nil {
		return manifest{}, err
	}
	return m, nil
}

func expectedAssets(version string, targets []Target) ([]string, error) {
	if len(targets) == 0 {
		return nil, fmt.Errorf("no targets")
	}
	names := []string{
		fmt.Sprintf("kigumi_%s_headers.tar.gz", version),
		fmt.Sprintf("kigumi_%s_source.tar.gz", version),
		sbomName,
		provName,
	}
	for _, t := range targets {
		names = append(names, fmt.Sprintf("kigumi_%s_%s.tar.gz", version, t.AssetSuffix()))
	}
	sort.Strings(names)
	return names, nil
}

// assetFiles lists the regular files in dir. Directories and non-regular
// entries are release mistakes, not ignored.
func assetFiles(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []os.DirEntry
	for _, e := range entries {
		if !e.Type().IsRegular() {
			return nil, fmt.Errorf("%s/%s is not a regular file", dir, e.Name())
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func writeManifest(dir string, m manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, manifestName), data, 0o644); err != nil {
		return err
	}
	// The checksum file covers every published byte except itself, including
	// the manifest.
	names := []string{manifestName}
	for _, a := range m.Assets {
		names = append(names, a.Name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		sum, _, err := hashFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, name)
	}
	return os.WriteFile(filepath.Join(dir, checksumsName), []byte(b.String()), 0o644)
}

func runVerify(args []string) error {
	fs := newFlagSet("verify")
	dir := fs.String("dir", "dist", "release asset directory")
	root := fs.String("root", ".", "repository root")
	version := fs.String("version", "", "release version (without v)")
	tag := fs.String("tag", "", "release tag")
	commit := fs.String("commit", "", "source commit")
	goVersion := fs.String("go-version", "", "expected Go version (optional)")
	targetsFile := fs.String("targets-file", "targets.txt", "target list file")
	repository := fs.String("repository", "kigumi-org/kigumi", "source repository")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *version == "" || *tag == "" || *commit == "" {
		return fmt.Errorf("verify: --version, --tag and --commit are required")
	}
	targets, err := readTargets(*targetsFile)
	if err != nil {
		return err
	}
	return verifyRelease(verifyInputs{
		Dir:        *dir,
		Root:       *root,
		Version:    *version,
		Tag:        *tag,
		Commit:     *commit,
		GoVersion:  *goVersion,
		Repository: *repository,
		Targets:    targets,
	})
}

type verifyInputs struct {
	Dir        string
	Root       string
	Version    string
	Tag        string
	Commit     string
	GoVersion  string
	Repository string
	Targets    []Target
}

func verifyRelease(in verifyInputs) error {
	if in.Repository == "" {
		in.Repository = "kigumi-org/kigumi"
	}
	vi, err := readVersion(in.Root)
	if err != nil {
		return err
	}
	if in.Version != vi.Release || in.Tag != vi.Tag {
		return fmt.Errorf("requested %s/%s, version.go has %s/%s", in.Tag, in.Version, vi.Tag, vi.Release)
	}
	data, err := os.ReadFile(filepath.Join(in.Dir, manifestName))
	if err != nil {
		return err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("%s: %w", manifestName, err)
	}
	if m.FormatVersion != 1 || m.Kind != "release-manifest" {
		return fmt.Errorf("%s: unsupported format %d kind %q", manifestName, m.FormatVersion, m.Kind)
	}
	if m.Release != in.Version || m.Tag != in.Tag || m.Commit != in.Commit {
		return fmt.Errorf("%s: identity %s/%s/%s does not match %s/%s/%s",
			manifestName, m.Release, m.Tag, m.Commit, in.Version, in.Tag, in.Commit)
	}
	if m.Repository != in.Repository {
		return fmt.Errorf("%s: repository %q, want %q", manifestName, m.Repository, in.Repository)
	}
	if !commitPattern.MatchString(m.Commit) || strings.TrimSpace(m.GoVersion) == "" {
		return fmt.Errorf("%s: incomplete build provenance", manifestName)
	}
	if in.GoVersion != "" && m.GoVersion != in.GoVersion {
		return fmt.Errorf("%s: go version %q, want %q", manifestName, m.GoVersion, in.GoVersion)
	}
	if m.ABIRevision != vi.ABIRevision || m.CanonicalABIVersion != vi.CanonicalABIVersion {
		return fmt.Errorf("%s: ABI %d/%s does not match version.go %d/%s",
			manifestName, m.ABIRevision, m.CanonicalABIVersion, vi.ABIRevision, vi.CanonicalABIVersion)
	}
	if strings.Join(m.Targets, ",") != strings.Join(targetNames(in.Targets), ",") {
		return fmt.Errorf("%s: targets %v, want %v", manifestName, m.Targets, targetNames(in.Targets))
	}
	if m.SBOM != sbomName || m.Provenance != provName {
		return fmt.Errorf("%s: sbom/provenance names %q/%q", manifestName, m.SBOM, m.Provenance)
	}
	expected, err := expectedAssets(in.Version, in.Targets)
	if err != nil {
		return err
	}
	manifestNames := make([]string, len(m.Assets))
	for i, a := range m.Assets {
		manifestNames[i] = a.Name
	}
	if strings.Join(manifestNames, "\n") != strings.Join(expected, "\n") {
		return fmt.Errorf("%s: assets %v, want %v", manifestName, manifestNames, expected)
	}
	sums, err := readChecksums(filepath.Join(in.Dir, checksumsName))
	if err != nil {
		return err
	}
	want := append([]string{manifestName}, expected...)
	sort.Strings(want)
	got := make([]string, 0, len(sums))
	for name := range sums {
		got = append(got, name)
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		return fmt.Errorf("%s covers %v, want %v", checksumsName, got, want)
	}
	entries, err := assetFiles(in.Dir)
	if err != nil {
		return err
	}
	allowed := map[string]bool{manifestName: true, checksumsName: true}
	for _, name := range expected {
		allowed[name] = true
	}
	for _, e := range entries {
		if !allowed[e.Name()] {
			return fmt.Errorf("%s: unexpected file %s", in.Dir, e.Name())
		}
	}
	for _, a := range m.Assets {
		path := filepath.Join(in.Dir, a.Name)
		sum, size, err := hashFile(path)
		if err != nil {
			return err
		}
		if sum != a.SHA256 || size != a.Size {
			return fmt.Errorf("%s: manifest digest/size mismatch (disk %s/%d, manifest %s/%d)", a.Name, sum, size, a.SHA256, a.Size)
		}
		if sums[a.Name] != a.SHA256 {
			return fmt.Errorf("%s: checksum file says %s, manifest says %s", a.Name, sums[a.Name], a.SHA256)
		}
	}
	if err := validateSBOM(filepath.Join(in.Dir, sbomName), m); err != nil {
		return err
	}
	if err := validateProvenance(filepath.Join(in.Dir, provName), m); err != nil {
		return err
	}
	return nil
}

func readChecksums(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}
		parts := strings.SplitN(text, "  ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("%s:%d: want <sha256>  <name>", path, line)
		}
		sum, name := parts[0], parts[1]
		if len(sum) != 64 {
			return nil, fmt.Errorf("%s:%d: %q is not a SHA-256", path, line, sum)
		}
		if _, err := hex.DecodeString(sum); err != nil {
			return nil, fmt.Errorf("%s:%d: %q is not hex", path, line, sum)
		}
		if name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
			return nil, fmt.Errorf("%s:%d: unsafe asset name %q", path, line, name)
		}
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("%s:%d: duplicate entry %q", path, line, name)
		}
		out[name] = sum
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func validateSBOM(path string, m manifest) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc spdxDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", sbomName, err)
	}
	if doc.SPDXVersion != "SPDX-2.3" || doc.SPDXID != "SPDXRef-DOCUMENT" {
		return fmt.Errorf("%s: not an SPDX 2.3 document", sbomName)
	}
	if !strings.Contains(doc.CreationInfo.Comment, "commit="+m.Commit) {
		return fmt.Errorf("%s: comment %q does not name commit %s", sbomName, doc.CreationInfo.Comment, m.Commit)
	}
	if !strings.Contains(doc.CreationInfo.Comment, "dirty=false") {
		return fmt.Errorf("%s: document does not record a clean build", sbomName)
	}
	if !strings.Contains(doc.CreationInfo.Comment, "go="+m.GoVersion) {
		return fmt.Errorf("%s: comment %q does not name Go %s", sbomName, doc.CreationInfo.Comment, m.GoVersion)
	}
	found := false
	for _, p := range doc.Packages {
		if p.Name == "kigumi" && p.VersionInfo == m.Release {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%s: no kigumi package at version %s", sbomName, m.Release)
	}
	if len(doc.Packages) < 2 {
		return fmt.Errorf("%s: no dependency packages", sbomName)
	}
	return nil
}

func validateProvenance(path string, m manifest) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc provenanceDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("%s: %w", provName, err)
	}
	if doc.Type != "https://in-toto.io/Statement/v1" || doc.PredicateType != "https://slsa.dev/provenance/v1" {
		return fmt.Errorf("%s: unexpected statement types", provName)
	}
	deps := doc.Predicate.BuildDefinition.ResolvedDependencies
	if len(deps) != 1 || deps[0].Digest["gitCommit"] != m.Commit {
		return fmt.Errorf("%s: resolved dependency does not name commit %s", provName, m.Commit)
	}
	if doc.Predicate.BuildDefinition.ExternalParameters.Tag != m.Tag {
		return fmt.Errorf("%s: tag %q, want %q", provName, doc.Predicate.BuildDefinition.ExternalParameters.Tag, m.Tag)
	}
	if len(doc.Subject) == 0 {
		return fmt.Errorf("%s: no subjects", provName)
	}
	byName := map[string]manifestAsset{}
	for _, a := range m.Assets {
		byName[a.Name] = a
	}
	for _, s := range doc.Subject {
		a, ok := byName[s.Name]
		if !ok {
			return fmt.Errorf("%s: subject %s is not a release asset", provName, s.Name)
		}
		if s.Digest["sha256"] != a.SHA256 {
			return fmt.Errorf("%s: subject %s digest differs from manifest", provName, s.Name)
		}
	}
	return nil
}

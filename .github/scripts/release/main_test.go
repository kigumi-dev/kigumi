package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/buildinfo"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

const testVersion = "0.1.0"
const testTag = "v0.1.0"
const testCommit = "0123456789abcdef0123456789abcdef01234567"
const testGo = "go1.26.1"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func versionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, versionFile), `package version

const Toolchain = "0.1"

const Release = "0.1.0"

const Zig = "0.17.0"

const (
	AbiRevision = 1
	CanonicalABIVersion = "0.3"
)
`)
	return root
}

func testTargets(t *testing.T, dir string) ([]Target, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "targets.txt")
	writeFile(t, path, "# test targets\ndarwin/arm64\nlinux/amd64\n")
	targets, err := readTargets(path)
	if err != nil {
		t.Fatal(err)
	}
	return targets, path
}

func fixtureDir(t *testing.T) (dir string, targets []Target, targetsFile string) {
	t.Helper()
	dir = t.TempDir()
	targets, targetsFile = testTargets(t, dir)
	_ = dir
	names, err := expectedAssets(testVersion, targets)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		switch name {
		case sbomName, provName:
			continue
		}
		writeFile(t, filepath.Join(dir, name), "contents of "+name)
	}
	doc := spdxDocument{
		SPDXVersion: "SPDX-2.3",
		DataLicense: "CC0-1.0",
		SPDXID:      "SPDXRef-DOCUMENT",
		Name:        "kigumi-" + testVersion,
		CreationInfo: spdxCreationInfo{
			Created:  "2026-11-11T00:00:00Z",
			Creators: []string{"Tool: kigumi-release"},
			Comment:  "commit=" + testCommit + ";dirty=false;go=" + testGo,
		},
		Packages: []spdxPackage{
			{SPDXID: toolPackageID, Name: "kigumi", VersionInfo: testVersion},
			{SPDXID: "SPDXRef-Package-dep", Name: "example.com/dep", VersionInfo: "v1.2.3"},
		},
	}
	if err := writeJSON(filepath.Join(dir, sbomName), doc); err != nil {
		t.Fatal(err)
	}
	prov, err := buildProvenance(dir, filepath.Join(dir, provName), provenanceInputs{
		Repository: "kigumi-org/kigumi",
		Ref:        "refs/tags/" + testTag,
		Commit:     testCommit,
		Workflow:   "kigumi-org/kigumi/.github/workflows/release.yml@refs/tags/" + testTag,
		RunID:      "42",
		Version:    testVersion,
		Tag:        testTag,
		Targets:    targetNames(targets),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(dir, provName), prov); err != nil {
		t.Fatal(err)
	}
	return dir, targets, targetsFile
}

func generate(t *testing.T, dir, root, targetsFile string) manifest {
	t.Helper()
	targets, err := readTargets(targetsFile)
	if err != nil {
		t.Fatal(err)
	}
	m, err := generateManifest(generateInputs{
		Dir:        dir,
		Root:       root,
		Version:    testVersion,
		Tag:        testTag,
		Commit:     testCommit,
		GoVersion:  testGo,
		Repository: "kigumi-org/kigumi",
		Targets:    targets,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	return m
}

func verify(t *testing.T, dir, root, targetsFile string) error {
	t.Helper()
	targets, err := readTargets(targetsFile)
	if err != nil {
		t.Fatal(err)
	}
	return verifyRelease(verifyInputs{
		Dir:        dir,
		Root:       root,
		Version:    testVersion,
		Tag:        testTag,
		Commit:     testCommit,
		GoVersion:  testGo,
		Repository: "kigumi-org/kigumi",
		Targets:    targets,
	})
}

func TestGenerateAndVerify(t *testing.T) {
	dir, _, targetsFile := fixtureDir(t)
	root := versionRoot(t)
	m := generate(t, dir, root, targetsFile)
	if m.ABIRevision != 1 || m.CanonicalABIVersion != "0.3" {
		t.Fatalf("manifest ABI = %d/%s", m.ABIRevision, m.CanonicalABIVersion)
	}
	if len(m.Targets) != 2 || m.Targets[0] != "darwin/arm64" {
		t.Fatalf("manifest targets = %v", m.Targets)
	}
	if err := verify(t, dir, root, targetsFile); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestGenerateRejectsMissingAsset(t *testing.T) {
	dir, _, targetsFile := fixtureDir(t)
	if err := os.Remove(filepath.Join(dir, "kigumi_0.1.0_linux_amd64.tar.gz")); err != nil {
		t.Fatal(err)
	}
	targets, _ := readTargets(targetsFile)
	_, err := generateManifest(generateInputs{Dir: dir, Root: versionRoot(t), Version: testVersion, Tag: testTag, Commit: testCommit, GoVersion: testGo, Repository: "kigumi-org/kigumi", Targets: targets})
	if err == nil || !strings.Contains(err.Error(), "missing release asset") {
		t.Fatalf("want missing asset error, got %v", err)
	}
}

func TestGenerateRejectsExtraAsset(t *testing.T) {
	dir, _, targetsFile := fixtureDir(t)
	writeFile(t, filepath.Join(dir, "kigumi_0.1.0_riscv64_linux.tar.gz"), "extra")
	targets, _ := readTargets(targetsFile)
	_, err := generateManifest(generateInputs{Dir: dir, Root: versionRoot(t), Version: testVersion, Tag: testTag, Commit: testCommit, GoVersion: testGo, Repository: "kigumi-org/kigumi", Targets: targets})
	if err == nil || !strings.Contains(err.Error(), "unexpected file") {
		t.Fatalf("want unexpected file error, got %v", err)
	}
}

func TestGenerateRejectsEmptyAssetAndBadIdentity(t *testing.T) {
	dir, _, targetsFile := fixtureDir(t)
	writeFile(t, filepath.Join(dir, "kigumi_0.1.0_source.tar.gz"), "")
	targets, _ := readTargets(targetsFile)
	base := generateInputs{Dir: dir, Root: versionRoot(t), Version: testVersion, Tag: testTag, Commit: testCommit, GoVersion: testGo, Repository: "kigumi-org/kigumi", Targets: targets}
	if _, err := generateManifest(base); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want empty asset error, got %v", err)
	}
	writeFile(t, filepath.Join(dir, "kigumi_0.1.0_source.tar.gz"), "ok")
	bad := base
	bad.Tag = "v0.1.1"
	if _, err := generateManifest(bad); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("want tag mismatch error, got %v", err)
	}
	bad = base
	bad.Commit = "not-a-commit"
	if _, err := generateManifest(bad); err == nil || !strings.Contains(err.Error(), "git object id") {
		t.Fatalf("want commit error, got %v", err)
	}
}

func TestGenerateRejectsDirtySBOM(t *testing.T) {
	dir, targets, _ := fixtureDir(t)
	var doc spdxDocument
	data, err := os.ReadFile(filepath.Join(dir, sbomName))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc.CreationInfo.Comment = "commit=" + testCommit + ";dirty=true;go=" + testGo
	if err := writeJSON(filepath.Join(dir, sbomName), doc); err != nil {
		t.Fatal(err)
	}
	_, err = generateManifest(generateInputs{Dir: dir, Root: versionRoot(t), Version: testVersion, Tag: testTag, Commit: testCommit, GoVersion: testGo, Repository: "kigumi-org/kigumi", Targets: targets})
	if err == nil || !strings.Contains(err.Error(), "clean build") {
		t.Fatalf("want dirty build error, got %v", err)
	}
}

func TestVerifyDetectsTamperingAndExtras(t *testing.T) {
	dir, _, targetsFile := fixtureDir(t)
	root := versionRoot(t)
	generate(t, dir, root, targetsFile)
	if err := verify(t, dir, root, targetsFile); err != nil {
		t.Fatalf("initial verify: %v", err)
	}
	asset := filepath.Join(dir, "kigumi_0.1.0_source.tar.gz")
	original, err := os.ReadFile(asset)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, asset, "tampered")
	if err := verify(t, dir, root, targetsFile); err == nil {
		t.Fatal("verify accepted a tampered asset")
	}
	writeFile(t, asset, string(original))
	if err := verify(t, dir, root, targetsFile); err != nil {
		t.Fatalf("verify after restore: %v", err)
	}
	writeFile(t, filepath.Join(dir, "unexpected.txt"), "x")
	if err := verify(t, dir, root, targetsFile); err == nil || !strings.Contains(err.Error(), "unexpected file") {
		t.Fatalf("want unexpected file error, got %v", err)
	}
	os.Remove(filepath.Join(dir, "unexpected.txt"))
	writeFile(t, filepath.Join(dir, checksumsName), "bad")
	if err := verify(t, dir, root, targetsFile); err == nil {
		t.Fatal("verify accepted a malformed checksum file")
	}
	// A well-formed checksum line that disagrees with the manifest must fail
	// too, even though the file on disk matches the manifest.
	generate(t, dir, root, targetsFile)
	good, err := os.ReadFile(filepath.Join(dir, checksumsName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(good), "\n"), "\n")
	if len(lines[0]) < 66 || lines[0][64:66] != "  " {
		t.Fatalf("unexpected checksum line %q", lines[0])
	}
	lines[0] = strings.Repeat("0", 64) + lines[0][64:]
	writeFile(t, filepath.Join(dir, checksumsName), strings.Join(lines, "\n")+"\n")
	if err := verify(t, dir, root, targetsFile); err == nil {
		t.Fatal("verify accepted a checksum that disagrees with the manifest")
	}
	generate(t, dir, root, targetsFile)
	if err := verify(t, dir, root, targetsFile); err != nil {
		t.Fatal(err)
	}
	// A wrong requested commit must not verify against the manifest.
	targets, _ := readTargets(targetsFile)
	err = verifyRelease(verifyInputs{
		Dir: dir, Root: root, Version: testVersion, Tag: testTag,
		Commit: strings.Repeat("a", 40), GoVersion: testGo,
		Repository: "kigumi-org/kigumi", Targets: targets,
	})
	if err == nil {
		t.Fatal("verify accepted a wrong commit")
	}
}

func TestReadVersionMissingConstant(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, versionFile), `package version
const Release = "0.1.0"
`)
	if _, err := readVersion(root); err == nil {
		t.Fatal("want error for missing constants")
	}
}

func TestReadVersionRejectsTagMismatch(t *testing.T) {
	root := versionRoot(t)
	v, err := readVersion(root)
	if err != nil {
		t.Fatal(err)
	}
	if v.Release != "0.1.0" || v.Tag != "v0.1.0" || v.ABIRevision != 1 {
		t.Fatalf("version = %+v", v)
	}
	if err := runVersion([]string{"--root", root, "--expect", "v0.2.0"}); err == nil {
		t.Fatal("want mismatch error")
	}
}

func TestReadTargetsValidation(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.txt")
	writeFile(t, bad, "linux\n")
	if _, err := readTargets(bad); err == nil {
		t.Fatal("want format error")
	}
	writeFile(t, bad, "linux/amd64\nlinux/amd64\n")
	if _, err := readTargets(bad); err == nil {
		t.Fatal("want duplicate error")
	}
	writeFile(t, bad, "linux/amd64\ndarwin/arm64\n")
	if _, err := readTargets(bad); err == nil {
		t.Fatal("want sort error")
	}
}

func TestBuildSBOMFromBuildInfo(t *testing.T) {
	info := &buildinfo.BuildInfo{
		GoVersion: "go1.26.1",
		Path:      "kigumi",
		Main:      debug.Module{Path: "kigumi", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: "github.com/spf13/cobra", Version: "v1.10.2", Sum: "h1:x"},
			{Path: "golang.org/x/text", Version: "v0.36.0", Sum: "h1:y"},
			{Path: "example.com/local", Version: "(devel)"},
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: testCommit},
			{Key: "vcs.modified", Value: "false"},
			{Key: "-trimpath", Value: "true"},
		},
	}
	doc, err := buildSBOMFromInfos(sbomOptions{
		Version: testVersion, Tag: testTag, Commit: testCommit,
		Repository: "kigumi-org/kigumi", Created: "2026-11-11T00:00:00Z",
	}, []*buildinfo.BuildInfo{info})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(doc.Packages); got != 3 {
		t.Fatalf("packages = %d, want main + 2 dependencies", got)
	}
	if doc.CreationInfo.Created != "2026-11-11T00:00:00Z" {
		t.Fatalf("created = %q", doc.CreationInfo.Created)
	}
	var purls []string
	for _, p := range doc.Packages {
		for _, r := range p.ExternalRefs {
			purls = append(purls, r.ReferenceLocator)
		}
	}
	want := []string{
		"pkg:golang/github.com/kigumi-org/kigumi@0.1.0",
		"pkg:golang/github.com/spf13/cobra@v1.10.2",
		"pkg:golang/golang.org/x/text@v0.36.0",
	}
	if !reflect.DeepEqual(purls, want) {
		t.Fatalf("purls = %v, want %v", purls, want)
	}
}

func TestBuildSBOMRejectsDirtyOrMismatchedBinaries(t *testing.T) {
	dirty := &buildinfo.BuildInfo{
		GoVersion: testGo, Path: "kigumi", Main: debug.Module{Path: "kigumi"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: testCommit},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	opts := sbomOptions{Version: testVersion, Tag: testTag, Commit: testCommit, Created: "2026-11-11T00:00:00Z"}
	if _, err := buildSBOMFromInfos(opts, []*buildinfo.BuildInfo{dirty}); err == nil {
		t.Fatal("want dirty build error")
	}
	missing := &buildinfo.BuildInfo{GoVersion: testGo, Path: "kigumi", Main: debug.Module{Path: "kigumi"}}
	if _, err := buildSBOMFromInfos(opts, []*buildinfo.BuildInfo{missing}); err == nil {
		t.Fatal("want missing revision error")
	}
	otherVal := *dirty
	other := &otherVal
	other.Settings = []debug.BuildSetting{
		{Key: "vcs.revision", Value: strings.Repeat("a", 40)},
		{Key: "vcs.modified", Value: "false"},
	}
	if _, err := buildSBOMFromInfos(opts, []*buildinfo.BuildInfo{other}); err == nil {
		t.Fatal("want commit mismatch error")
	}
}

func readTar(t *testing.T, path string) map[string]*tar.Header {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out := map[string]*tar.Header{}
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		h := *hdr
		hdr = &h
		out[hdr.Name] = hdr
		if hdr.Typeflag == tar.TypeReg && len(body) != int(hdr.Size) {
			t.Fatalf("%s: read %d bytes, header says %d", hdr.Name, len(body), hdr.Size)
		}
	}
	return out
}

func TestArchiveIsDeterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "b.txt"), "bravo")
	writeFile(t, filepath.Join(root, "a.sh"), "#!/bin/sh\necho hi\n")
	if err := os.Chmod(filepath.Join(root, "a.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "dir", "c.txt"), "charlie")
	if err := os.Symlink("a.sh", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	entries, err := collectEntries(root, false, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(entries); got != 4 {
		t.Fatalf("entries = %d, want 4", got)
	}
	out1 := filepath.Join(t.TempDir(), "one.tar.gz")
	out2 := filepath.Join(t.TempDir(), "two.tar.gz")
	if err := writeArchive(out1, "pkg-1.0", entries); err != nil {
		t.Fatal(err)
	}
	// Touch a source file: reproducibility must not depend on mtime.
	future := time.Now().Add(24 * time.Hour)
	if err := os.Chtimes(filepath.Join(root, "b.txt"), future, future); err != nil {
		t.Fatal(err)
	}
	if err := writeArchive(out2, "pkg-1.0", entries); err != nil {
		t.Fatal(err)
	}
	b1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatal("archive bytes differ between identical inputs")
	}
	hdrs := readTar(t, out1)
	for name, hdr := range hdrs {
		if !strings.HasPrefix(name, "pkg-1.0/") {
			t.Fatalf("entry %q lost the prefix", name)
		}
		if !hdr.ModTime.Equal(time.Unix(0, 0)) {
			t.Fatalf("entry %q mtime = %v", name, hdr.ModTime)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Fatalf("entry %q owner = %d/%d", name, hdr.Uid, hdr.Gid)
		}
	}
	if hdrs["pkg-1.0/a.sh"].Mode != 0o755 {
		t.Fatalf("executable mode = %o", hdrs["pkg-1.0/a.sh"].Mode)
	}
	if hdrs["pkg-1.0/b.txt"].Mode != 0o644 {
		t.Fatalf("file mode = %o", hdrs["pkg-1.0/b.txt"].Mode)
	}
	if hdrs["pkg-1.0/link"].Linkname != "a.sh" {
		t.Fatalf("symlink target = %q", hdrs["pkg-1.0/link"].Linkname)
	}
}

func TestArchiveRejectsPathsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := collectEntries(root, false, false, []string{"../escape"}); err == nil {
		t.Fatal("want escape error")
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestArchiveGitIncludesOnlyTrackedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	writeFile(t, filepath.Join(root, "tracked.txt"), "tracked")
	writeFile(t, filepath.Join(root, "nested", "also.txt"), "also")
	gitRun(t, root, "add", "tracked.txt", "nested/also.txt")
	gitRun(t, root, "commit", "-q", "-m", "fixture")
	writeFile(t, filepath.Join(root, "untracked.txt"), "untracked")
	entries, err := collectEntries(root, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.name)
	}
	if !reflect.DeepEqual(names, []string{"nested/also.txt", "tracked.txt"}) {
		t.Fatalf("git entries = %v", names)
	}
}

func TestValidateSBOMRequiresDependencies(t *testing.T) {
	dir := t.TempDir()
	doc := spdxDocument{
		SPDXVersion: "SPDX-2.3",
		SPDXID:      "SPDXRef-DOCUMENT",
		CreationInfo: spdxCreationInfo{
			Comment: "commit=" + testCommit + ";dirty=false",
		},
		Packages: []spdxPackage{{Name: "kigumi", VersionInfo: testVersion}},
	}
	if err := writeJSON(filepath.Join(dir, sbomName), doc); err != nil {
		t.Fatal(err)
	}
	m := manifest{Release: testVersion, Commit: testCommit}
	if err := validateSBOM(filepath.Join(dir, sbomName), m); err == nil {
		t.Fatal("want no-dependencies error")
	}
}

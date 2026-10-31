package fetch_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"kigumi/internal/devkit/zigrelease"
	"kigumi/internal/toolchain/zig/fetch"
	"kigumi/internal/toolchain/zig/store"
)

// fakeTarball builds a tar.gz stand-in; real releases are tar.xz, but gzip
// needs no extra tooling and `tar` auto-detects either.
func fakeTarball(t *testing.T, triple, version, zigScript string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	top := fmt.Sprintf("zig-%s-%s", triple, version)
	entries := []struct {
		name string
		body string
		mode int64
		dir  bool
	}{
		{name: top + "/", mode: 0o755, dir: true},
		{name: top + "/zig", body: zigScript, mode: 0o755},
		{name: top + "/README.md", body: "fake zig release for tests\n", mode: 0o644},
	}
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: e.mode, Size: int64(len(e.body))}
		if e.dir {
			hdr.Typeflag = tar.TypeDir
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// fakeServer signs with a freshly generated test key; publicKey is that
// key's line for Options.PublicKey — the real pinned key is never touched.
func fakeServer(t *testing.T, version, triple string, tarball []byte, corruptSig, corruptChecksum bool) (indexURL, publicKey string, cleanup func()) {
	t.Helper()
	priv, keyID, pubLine := fetchtest.NewSigningKey()
	sum := sha256.Sum256(tarball)
	shasum := hex.EncodeToString(sum[:])
	if corruptChecksum {
		shasum = strings.Repeat("0", len(shasum))
	}
	tarName := fmt.Sprintf("zig-%s-%s.tar.gz", triple, version)
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		idx := map[string]map[string]any{
			version: {triple: map[string]string{
				"tarball": srv.URL + "/" + tarName,
				"shasum":  shasum,
				"size":    fmt.Sprint(len(tarball)),
			}},
		}
		json.NewEncoder(w).Encode(idx)
	})
	mux.HandleFunc("/"+tarName, func(w http.ResponseWriter, r *http.Request) {
		w.Write(tarball)
	})
	mux.HandleFunc("/"+tarName+".minisig", func(w http.ResponseWriter, r *http.Request) {
		signed := tarball
		if corruptSig {
			signed = append([]byte{0}, tarball...)
		}
		sig := fetchtest.SignFile(priv, keyID, signed, "timestamp:1 file:"+tarName+" hashed")
		w.Write(sig)
	})
	srv = httptest.NewServer(mux)
	return srv.URL + "/index.json", pubLine, srv.Close
}

func TestInstallReleaseDownloadsVerifiesAndInstalls(t *testing.T) {
	triple, err := fetch.HostTriple()
	if err != nil {
		t.Skip(err)
	}
	tarball := fakeTarball(t, triple, "9.9.9", "#!/bin/sh\necho 9.9.9\n")
	indexURL, pubKey, cleanup := fakeServer(t, "9.9.9", triple, tarball, false, false)
	defer cleanup()

	root := t.TempDir()
	m, err := fetch.InstallRelease(fetch.Options{Lookup: exec.LookPath, IndexURL: indexURL, StoreRoot: root, Version: "9.9.9", PublicKey: pubKey})
	if err != nil {
		t.Fatalf("InstallRelease: %v", err)
	}
	if m.Version != "9.9.9" || m.Obtained != "release" {
		t.Fatalf("manifest = %+v", m)
	}
	path, ok := store.Installed(root, "9.9.9")
	if !ok {
		t.Fatal("not installed after InstallRelease")
	}
	out, err := exec.Command(path, "version").Output()
	if err != nil || strings.TrimSpace(string(out)) != "9.9.9" {
		t.Fatalf("installed zig: out=%q err=%v", out, err)
	}
}

func TestInstallReleaseRejectsChecksumMismatch(t *testing.T) {
	triple, err := fetch.HostTriple()
	if err != nil {
		t.Skip(err)
	}
	tarball := fakeTarball(t, triple, "9.9.8", "#!/bin/sh\necho 9.9.8\n")
	indexURL, pubKey, cleanup := fakeServer(t, "9.9.8", triple, tarball, false, true)
	defer cleanup()

	root := t.TempDir()
	if _, err := fetch.InstallRelease(fetch.Options{Lookup: exec.LookPath, IndexURL: indexURL, StoreRoot: root, Version: "9.9.8", PublicKey: pubKey}); err == nil {
		t.Fatal("expected a checksum mismatch error")
	}
	if _, ok := store.Installed(root, "9.9.8"); ok {
		t.Fatal("a checksum failure must not leave a partial install")
	}
}

func TestInstallReleaseRejectsBadSignature(t *testing.T) {
	triple, err := fetch.HostTriple()
	if err != nil {
		t.Skip(err)
	}
	tarball := fakeTarball(t, triple, "9.9.7", "#!/bin/sh\necho 9.9.7\n")
	indexURL, pubKey, cleanup := fakeServer(t, "9.9.7", triple, tarball, true, false)
	defer cleanup()

	root := t.TempDir()
	if _, err := fetch.InstallRelease(fetch.Options{Lookup: exec.LookPath, IndexURL: indexURL, StoreRoot: root, Version: "9.9.7", PublicKey: pubKey}); err == nil {
		t.Fatal("expected a signature verification error")
	}
	if _, ok := store.Installed(root, "9.9.7"); ok {
		t.Fatal("a signature failure must not leave a partial install")
	}
}

func TestInstallReleaseNoPrebuiltForDevSnapshot(t *testing.T) {
	indexURL, pubKey, cleanup := fakeServer(t, "9.9.6", "x86_64-linux", []byte("unused"), false, false)
	defer cleanup()
	root := t.TempDir()
	_, err := fetch.InstallRelease(fetch.Options{Lookup: exec.LookPath, IndexURL: indexURL, StoreRoot: root, Version: "0.16.0-dev.2287+eb3f16db5", GOOS: "linux", GOARCH: "amd64", PublicKey: pubKey})
	if err == nil {
		t.Fatal("expected ErrNoPrebuilt")
	}
}

func TestInstallReleaseWithoutLookupFailsToExtract(t *testing.T) {
	triple, err := fetch.HostTriple()
	if err != nil {
		t.Skip(err)
	}
	tarball := fakeTarball(t, triple, "9.9.5", "#!/bin/sh\necho 9.9.5\n")
	indexURL, pubKey, cleanup := fakeServer(t, "9.9.5", triple, tarball, false, false)
	defer cleanup()

	root := t.TempDir()
	_, err = fetch.InstallRelease(fetch.Options{IndexURL: indexURL, StoreRoot: root, Version: "9.9.5", PublicKey: pubKey})
	if err == nil || !strings.Contains(err.Error(), "no tool search path given") {
		t.Fatalf("InstallRelease with a nil Lookup = %v, want the no tool search path error", err)
	}
	if _, ok := store.Installed(root, "9.9.5"); ok {
		t.Fatal("a failed extraction must not leave an install")
	}
}

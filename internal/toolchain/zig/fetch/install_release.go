package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"kigumi/internal/toolchain/zig/store"
)

// ErrNoPrebuilt marks a version/host with no official tarball (a dev
// snapshot or unsupported host); the caller should suggest `kigumi zig install --source`.
var ErrNoPrebuilt = errors.New("no prebuilt zig tarball for this version and host")

// Options configures InstallRelease; Client, IndexURL, GOOS and GOARCH
// default to http.DefaultClient, DefaultIndexURL and the running host.
type Options struct {
	Client       *http.Client
	IndexURL     string
	StoreRoot    string
	Version      string
	GOOS, GOARCH string
	// Stderr, when set, gets one progress line before the download starts.
	Stderr io.Writer
	// PublicKey overrides the pinned Zig signing key; tests only, a fake
	// server can't be verified against the real one.
	PublicKey string
	// Lookup finds tar; nil finds nothing.
	Lookup func(name string) (string, error)
}

// InstallRelease verifies version's tarball (sha256 and minisign) and
// installs it atomically, cleaning up staging on any failure after download.
func InstallRelease(opts Options) (store.Manifest, error) {
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	indexURL := opts.IndexURL
	if indexURL == "" {
		indexURL = DefaultIndexURL
	}
	goos, goarch := opts.GOOS, opts.GOARCH
	if goos == "" && goarch == "" {
		goos, goarch = runtime.GOOS, runtime.GOARCH
	}
	triple, err := Triple(goos, goarch)
	if err != nil {
		return store.Manifest{}, err
	}
	idx, err := FetchIndex(client, indexURL)
	if err != nil {
		return store.Manifest{}, err
	}
	tb, ok := idx.Tarball(opts.Version, triple)
	if !ok {
		return store.Manifest{}, fmt.Errorf("%s for %s: %w", opts.Version, triple, ErrNoPrebuilt)
	}
	if opts.Stderr != nil {
		fmt.Fprintf(opts.Stderr, "kigumi: fetching zig %s for %s...\n", opts.Version, triple)
	}

	tmpDir, err := os.MkdirTemp("", "kigumi-zig-fetch-")
	if err != nil {
		return store.Manifest{}, err
	}
	defer os.RemoveAll(tmpDir)
	tarPath := filepath.Join(tmpDir, filepath.Base(tb.Tarball))
	if err := downloadToFile(client, tb.Tarball, tarPath); err != nil {
		return store.Manifest{}, err
	}
	data, err := os.ReadFile(tarPath)
	if err != nil {
		return store.Manifest{}, err
	}
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != tb.Shasum {
		return store.Manifest{}, fmt.Errorf("%s: sha256 mismatch: index says %s, downloaded file is %s", tb.Tarball, tb.Shasum, hex.EncodeToString(sum[:]))
	}
	sigData, err := downloadBytes(client, tb.Tarball+".minisig")
	if err != nil {
		return store.Manifest{}, fmt.Errorf("download signature for %s: %w", tb.Tarball, err)
	}
	if err := verifyMinisign(data, sigData, opts.PublicKey); err != nil {
		return store.Manifest{}, fmt.Errorf("%s: %w", tb.Tarball, err)
	}

	st, err := store.Stage(opts.StoreRoot, opts.Version)
	if err != nil {
		return store.Manifest{}, err
	}
	defer st.Discard()
	if err := extractTarball(opts.Lookup, tarPath, st.Dir); err != nil {
		return store.Manifest{}, err
	}
	m := store.Manifest{
		Version:     opts.Version,
		OS:          goos,
		Arch:        goarch,
		TarballURL:  tb.Tarball,
		SHA256:      tb.Shasum,
		InstalledAt: time.Now().UTC(),
		Obtained:    "release",
	}
	if err := store.WriteManifest(st.Dir, m); err != nil {
		return store.Manifest{}, err
	}
	if err := st.Commit(); err != nil {
		return store.Manifest{}, err
	}
	return m, nil
}

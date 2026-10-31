package driver_test

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
	"os"
	"strings"
	"testing"

	"kigumi/internal/devkit/zigrelease"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/zig/fetch"
)

// fakeZigIndexServer signs its tarball with a throwaway key so an on-demand
// install never reaches ziglang.org; publicKey is never the real pinned key.
func fakeZigIndexServer(t *testing.T, version string) (indexURL, publicKey string, cleanup func()) {
	t.Helper()
	triple, err := fetch.HostTriple()
	if err != nil {
		t.Skip(err)
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	top := fmt.Sprintf("zig-%s-%s", triple, version)
	write := func(name, body string, mode int64, dir bool) {
		hdr := &tar.Header{Name: name, Mode: mode, Size: int64(len(body))}
		if dir {
			hdr.Typeflag = tar.TypeDir
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write(top+"/", "", 0o755, true)
	write(top+"/zig", "#!/bin/sh\nif [ \"$1\" = version ]; then echo "+version+"; exit 0; fi\nexit 1\n", 0o755, false)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buf.Bytes()

	priv, keyID, pubLine := fetchtest.NewSigningKey()
	sum := sha256.Sum256(tarball)
	tarName := fmt.Sprintf("zig-%s-%s.tar.gz", triple, version)
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		idx := map[string]map[string]any{version: {triple: map[string]string{
			"tarball": srv.URL + "/" + tarName,
			"shasum":  hex.EncodeToString(sum[:]),
			"size":    fmt.Sprint(len(tarball)),
		}}}
		json.NewEncoder(w).Encode(idx)
	})
	mux.HandleFunc("/"+tarName, func(w http.ResponseWriter, r *http.Request) { w.Write(tarball) })
	mux.HandleFunc("/"+tarName+".minisig", func(w http.ResponseWriter, r *http.Request) {
		w.Write(fetchtest.SignFile(priv, keyID, tarball, "timestamp:1 file:"+tarName+" hashed"))
	})
	srv = httptest.NewServer(mux)
	return srv.URL + "/index.json", pubLine, srv.Close
}

func TestResolveZigFetchesOnDemand(t *testing.T) {
	indexURL, pubKey, cleanup := fakeZigIndexServer(t, "9.5.1")
	defer cleanup()

	m, err := loadPinnedOpts(t, "9.5.1", "", driver.LoadOptions{Compiler: testCompiler(), ZigIndexURL: indexURL, ZigPublicKey: pubKey})
	if err != nil {
		t.Fatal(err)
	}
	path, err := m.ResolveZig()
	if err != nil {
		t.Fatalf("ResolveZig: %v", err)
	}
	if !strings.Contains(path, "9.5.1") {
		t.Fatalf("expected the store path to mention the fetched version, got %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestResolveZigFetchFailureReportsE1564(t *testing.T) {
	deadServer := httptest.NewServer(http.NotFoundHandler())
	deadServer.Close()

	m, err := loadPinnedOpts(t, "9.5.2", "", driver.LoadOptions{Compiler: testCompiler(), ZigIndexURL: deadServer.URL + "/index.json"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.ResolveZig()
	if err == nil || !strings.Contains(err.Error(), "E1564") {
		t.Fatalf("want E1564 (fetch unavailable), got: %v", err)
	}
}

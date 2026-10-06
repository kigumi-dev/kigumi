package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DefaultIndexURL is ziglang.org's published download index.
const DefaultIndexURL = "https://ziglang.org/download/index.json"

// Tarball is one os/arch release artifact's entry in the index.
type Tarball struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"`
	Size    string `json:"size"`
}

// versionEntry is one version's row in index.json, kept raw since the
// "<arch>-<os>" tarball field list isn't fixed.
type versionEntry map[string]json.RawMessage

// Index is the parsed index.json: version string (or "master") to its entry.
type Index map[string]versionEntry

// FetchIndex downloads and parses indexURL (DefaultIndexURL, or a mirror
// for --base-url and for tests).
func FetchIndex(client *http.Client, indexURL string) (Index, error) {
	req, err := http.NewRequest(http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", indexURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %s", indexURL, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", indexURL, err)
	}
	var idx Index
	if err := json.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parse %s: %w", indexURL, err)
	}
	return idx, nil
}

// Tarball finds version's entry for the index triple ("x86_64-linux"); ok is
// false for a dev snapshot or an unsupported host.
func (idx Index) Tarball(version, triple string) (Tarball, bool) {
	entry, ok := idx[version]
	if !ok {
		return Tarball{}, false
	}
	raw, ok := entry[triple]
	if !ok {
		return Tarball{}, false
	}
	var t Tarball
	if err := json.Unmarshal(raw, &t); err != nil {
		return Tarball{}, false
	}
	return t, true
}

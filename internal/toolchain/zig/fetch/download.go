package fetch

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// downloadToFile streams url's body into a new file at path, failing (and
// removing the partial file) on a non-200 response or a transfer error.
func downloadToFile(client *http.Client, url, path string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(path)
		return fmt.Errorf("download %s: %w", url, err)
	}
	return f.Close()
}

// downloadBytes is downloadToFile for a small file (a .minisig) read back
// into memory rather than kept on disk.
func downloadBytes(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

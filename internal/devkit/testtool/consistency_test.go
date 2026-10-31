package testtool_test

import (
	"reflect"
	"testing"

	"kigumi/internal/devkit/testtool"
)

// TestManifestConsistency cross-checks the checked-in manifest against
// the fixtures on disk and its own reason catalog.
func TestManifestConsistency(t *testing.T) {
	m, err := testtool.Load("../../../testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != testtool.SchemaVersion {
		t.Errorf("manifest schema %d, want %d", m.Schema, testtool.SchemaVersion)
	}
	if !reflect.DeepEqual(m.Reasons, testtool.Reasons) {
		t.Errorf("manifest reason catalog does not match testtool.Reasons; regenerate with UPDATE=1")
	}
	for _, err := range testtool.Consistency("../../..", m) {
		t.Error(err)
	}
}

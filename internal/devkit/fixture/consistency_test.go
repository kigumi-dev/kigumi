package fixture_test

import (
	"reflect"
	"testing"

	"kigumi/internal/devkit/fixture"
)

// TestManifestConsistency cross-checks the checked-in manifest against
// the fixtures on disk and its own reason catalog.
func TestManifestConsistency(t *testing.T) {
	m, err := fixture.Load("../../../testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != fixture.SchemaVersion {
		t.Errorf("manifest schema %d, want %d", m.Schema, fixture.SchemaVersion)
	}
	if !reflect.DeepEqual(m.Reasons, fixture.Reasons) {
		t.Errorf("manifest reason catalog does not match fixture.Reasons; regenerate with UPDATE=1")
	}
	for _, err := range fixture.Consistency("../../..", m) {
		t.Error(err)
	}
}

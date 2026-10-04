package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestFormatReportsOmittedDiagnostics checks that exceeding the bag's limit reports how many diagnostics were dropped, not a silent truncation.
func TestFormatReportsOmittedDiagnostics(t *testing.T) {
	var sb strings.Builder
	for range 150 {
		sb.WriteString("`\n")
	}
	_, err := driver.Format(driver.Source{Path: "a.kg", Src: []byte(sb.String())})
	if err == nil {
		t.Fatal("150 invalid lines must fail to format")
	}
	if !strings.Contains(err.Error(), "diagnostics omitted") {
		t.Fatalf("rendered report must mention omitted diagnostics, got:\n%s", err.Error())
	}
}

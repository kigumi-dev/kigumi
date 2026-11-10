package diag_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// The machine report's field names, order and values are a public contract:
// this golden pins them.
func TestMachineReportSchema(t *testing.T) {
	f := token.NewFile("pkg/main.kg", []byte("let x = foo bar baz\n"))
	d := diag.Errorf(diag.At(f, token.Span{Start: 12, End: 15}), "whitespace call chain is not allowed").
		WithNote(diag.At(f, token.Span{Start: 8, End: 11}), "callee is here").
		WithHelp("wrap the arguments in parentheses")
	d.Code = "E700"
	var buf bytes.Buffer
	if err := diag.Encode(&buf, diag.Report(f, []diag.Diagnostic{d})); err != nil {
		t.Fatal(err)
	}
	want := `{
  "schemaVersion": 1,
  "tool": "kigumi",
  "diagnostics": [
    {
      "severity": "error",
      "code": "E700",
      "message": "whitespace call chain is not allowed",
      "fingerprint": "` + diag.Fingerprint("E700", "pkg/main.kg", "whitespace call chain is not allowed") + `",
      "file": "pkg/main.kg",
      "span": {
        "start": {
          "line": 1,
          "column": 13,
          "offset": 12
        },
        "end": {
          "line": 1,
          "column": 16,
          "offset": 15
        }
      },
      "notes": [
        {
          "severity": "note",
          "message": "callee is here",
          "file": "pkg/main.kg",
          "span": {
            "start": {
              "line": 1,
              "column": 9,
              "offset": 8
            },
            "end": {
              "line": 1,
              "column": 12,
              "offset": 11
            }
          }
        }
      ],
      "help": [
        "wrap the arguments in parentheses"
      ]
    }
  ]
}
`
	if got := buf.String(); got != want {
		t.Errorf("machine report mismatch\n got: %s\nwant: %s", got, want)
	}
}

// A diagnostic with no source location (a release-policy finding) keeps its
// severity, code and message and omits file and span.
func TestMachineEntryWithoutLocation(t *testing.T) {
	d := diag.Diagnostic{Severity: diag.Warning, Code: "E2100", Msg: "unlocked dependency"}
	e := diag.Entry(nil, d)
	if e.File != "" || e.Span != nil {
		t.Errorf("location must be absent: %+v", e)
	}
	if e.Severity != "warning" || e.Code != "E2100" || e.Message != "unlocked dependency" {
		t.Errorf("entry = %+v", e)
	}
}

// The fingerprint ignores positions, so a baselined diagnostic that moves does
// not come back.
func TestFingerprintIgnoresPosition(t *testing.T) {
	a := token.NewFile("a.kg", []byte("aaaa\n"))
	b := token.NewFile("a.kg", []byte("bbbbbbbb\naaaa\n"))
	da := diag.Errorf(diag.At(a, token.Span{Start: 0, End: 1}), "boom")
	da.Code = "E1"
	db := diag.Errorf(diag.At(b, token.Span{Start: 9, End: 10}), "boom")
	db.Code = "E1"
	if ea, eb := diag.Entry(a, da), diag.Entry(b, db); ea.Fingerprint != eb.Fingerprint {
		t.Errorf("fingerprint changed with position: %s != %s", ea.Fingerprint, eb.Fingerprint)
	}
	dc := diag.Errorf(diag.At(a, token.Span{Start: 0, End: 1}), "other")
	dc.Code = "E1"
	if diag.Entry(a, da).Fingerprint == diag.Entry(a, dc).Fingerprint {
		t.Error("a different message must fingerprint differently")
	}
}

func TestSARIFMapping(t *testing.T) {
	f := token.NewFile("cmd/app/main.kg", []byte("let x = 1\n"))
	err := diag.Errorf(diag.At(f, token.Span{Start: 8, End: 9}), "no")
	err.Code = "E400"
	warn := diag.Warnf(diag.At(f, token.Span{Start: 4, End: 5}), "unused")
	point := diag.Errorf(diag.At(f, token.Span{Start: 0, End: 0}), "here")
	point.Code = "E401"
	log := diag.SARIF(diag.Report(f, []diag.Diagnostic{err, warn, point}))
	if log.Version != "2.1.0" || log.Runs[0].Tool.Driver.Name != "kigumi" {
		t.Fatalf("svc = %+v", log)
	}
	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 2 || rules[0].ID != "E400" || rules[1].ID != "E401" {
		t.Errorf("rules = %+v", rules)
	}
	res := log.Runs[0].Results
	if len(res) != 3 || res[0].Level != "error" || res[1].Level != "warning" || res[1].RuleID != "" {
		t.Errorf("results = %+v", res)
	}
	region := res[0].Locations[0].PhysicalLocation.Region
	if region.StartLine != 1 || region.StartColumn != 9 || region.EndLine != 1 || region.EndColumn != 10 {
		t.Errorf("region = %+v", region)
	}
	if r := res[2].Locations[0].PhysicalLocation.Region; r.StartColumn != 1 || r.EndColumn != 2 {
		t.Errorf("zero-width region = %+v", r)
	}
	if uri := res[0].Locations[0].PhysicalLocation.ArtifactLocation.URI; uri != "cmd/app/main.kg" {
		t.Errorf("artifact uri = %q", uri)
	}
}

func TestFilterAllowAndBaseline(t *testing.T) {
	e := diag.MachineDiagnostic{Severity: "warning", Code: "E700", Message: "m", File: "a.kg"}
	e.Fingerprint = diag.Fingerprint(e.Code, e.File, e.Message)
	other := diag.MachineDiagnostic{Severity: "warning", Code: "E701", Message: "m", File: "a.kg"}
	other.Fingerprint = diag.Fingerprint(other.Code, other.File, other.Message)
	f := diag.NewFilter([]string{"E700"}, map[string]bool{other.Fingerprint: true})
	if !f.Drop(e) || !f.Drop(other) {
		t.Errorf("both entries must be silenced: %+v", f)
	}
	if f.Drop(diag.MachineDiagnostic{Code: "E999", Message: "m"}) {
		t.Error("an unrelated code must survive")
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	f := token.NewFile("a.kg", []byte("let x = 1\n"))
	d := diag.Errorf(diag.At(f, token.Span{Start: 8, End: 9}), "boom")
	d.Code = "E1"
	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := diag.WriteBaseline(path, diag.Report(f, []diag.Diagnostic{d})); err != nil {
		t.Fatal(err)
	}
	base, err := diag.ReadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if !diag.NewFilter(nil, base).Drop(diag.Entry(f, d)) {
		t.Error("the written baseline must silence its own diagnostic")
	}
	// An empty baseline is a report with no diagnostics, not a null list.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rep diag.MachineReport
	if err := json.Unmarshal(b, &rep); err != nil || rep.SchemaVersion != diag.SchemaVersion {
		t.Fatalf("baseline = %s (%v)", b, err)
	}
}

func TestReadBaselineRejects(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte("{"), 0o644)
	if _, err := diag.ReadBaseline(bad); err == nil {
		t.Error("malformed baseline must be rejected")
	}
	newer := filepath.Join(dir, "newer.json")
	os.WriteFile(newer, []byte(`{"schemaVersion":99,"tool":"kigumi","diagnostics":[]}`), 0o644)
	if _, err := diag.ReadBaseline(newer); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("newer schema must be rejected: %v", err)
	}
}

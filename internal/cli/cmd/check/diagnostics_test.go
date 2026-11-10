package check_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
	"kigumi/internal/cli/exit"
	"kigumi/internal/lang/diag"
)

const typeError = "let x: String = 1\nprint x\n"

func stdRoot(t *testing.T) string {
	t.Helper()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	return std
}

// module writes one file into a fresh module directory and returns the root.
func module(t *testing.T, name, src string) string {
	t.Helper()
	t.Setenv("KIGUMI_PKG_CACHE", t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func execute(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	return clitest.Execute(t, append([]string{"--std", stdRoot(t)}, args...)...)
}

// The JSON report is the stable machine schema: the same shape, tool field and
// version on every run, one entry per kept diagnostic, and exit 1 because the
// error survives.
func TestCheckJSONSchema(t *testing.T) {
	dir := module(t, "main.kg", typeError)
	code, out, errOut := execute(t, "check", "--format", "json", dir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1; stderr %q", code, errOut)
	}
	if errOut != "" {
		t.Errorf("machine mode must not write human diagnostics: %q", errOut)
	}
	var rep diag.MachineReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("stdout is not the machine report: %v\n%s", err, out)
	}
	if rep.SchemaVersion != diag.SchemaVersion || rep.Tool != diag.ToolName {
		t.Fatalf("envelope = %+v", rep)
	}
	if len(rep.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v", rep.Diagnostics)
	}
	d := rep.Diagnostics[0]
	if d.Severity != "error" || d.Code != "E400" || d.File != "main.kg" {
		t.Errorf("diagnostic = %+v", d)
	}
	if d.Span == nil || d.Span.Start.Line != 1 || d.Span.Start.Offset != 16 {
		t.Errorf("span = %+v", d.Span)
	}
	if d.Fingerprint != diag.Fingerprint("E400", "main.kg", d.Message) {
		t.Errorf("fingerprint = %q", d.Fingerprint)
	}
	// A second run of the same source prints byte-identical output.
	_, again, _ := execute(t, "check", "--format", "json", dir)
	if again != out {
		t.Errorf("machine output changed between runs:\n%s\n---\n%s", out, again)
	}
}

// A clean module still prints a report, with an empty (not null) diagnostics
// list, and exits 0.
func TestCheckJSONCleanModule(t *testing.T) {
	dir := module(t, "main.kg", "print \"ok\"\n")
	code, out, errOut := execute(t, "check", "--format", "json", dir)
	if code != 0 || errOut != "" {
		t.Fatalf("exit = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "\"diagnostics\": []") {
		t.Errorf("clean report must carry an empty list: %s", out)
	}
}

// Syntax errors are diagnostics too: the machine report sees the catalog code
// and the parse span, and the command exits 1.
func TestCheckJSONParseError(t *testing.T) {
	dir := module(t, "main.kg", "fn main( -> Void {\n}\n")
	code, out, _ := execute(t, "check", "--format", "json", dir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	var rep diag.MachineReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Diagnostics) == 0 || rep.Diagnostics[0].Code != "E2199" {
		t.Fatalf("diagnostics = %+v", rep.Diagnostics)
	}
	notes := 0
	for _, d := range rep.Diagnostics {
		notes += len(d.Notes)
	}
	if notes == 0 {
		t.Error("the parse diagnostic's note must survive into the machine report")
	}
}

// SARIF is the same diagnostics under SARIF 2.1.0's names.
func TestCheckSARIF(t *testing.T) {
	dir := module(t, "main.kg", typeError)
	code, out, _ := execute(t, "check", "--format", "sarif", dir)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	var log struct {
		Schema  string `json:"$schema"`
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID  string `json:"ruleId"`
				Level   string `json:"level"`
				Message struct {
					Text string `json:"text"`
				} `json:"message"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI string `json:"uri"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine   int `json:"startLine"`
							StartColumn int `json:"startColumn"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal([]byte(out), &log); err != nil {
		t.Fatalf("stdout is not SARIF: %v\n%s", err, out)
	}
	if log.Version != "2.1.0" || log.Schema == "" {
		t.Errorf("log header = %+v", log)
	}
	run := log.Runs[0]
	if run.Tool.Driver.Name != "kigumi" || len(run.Tool.Driver.Rules) != 1 || run.Tool.Driver.Rules[0].ID != "E400" {
		t.Errorf("driver = %+v", run.Tool.Driver)
	}
	if len(run.Results) != 1 {
		t.Fatalf("results = %+v", run.Results)
	}
	r := run.Results[0]
	if r.RuleID != "E400" || r.Level != "error" || r.Message.Text == "" {
		t.Errorf("result = %+v", r)
	}
	loc := r.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "main.kg" || loc.Region.StartLine != 1 || loc.Region.StartColumn != 17 {
		t.Errorf("location = %+v", loc)
	}
}

// --deny-warnings turns a surviving warning into exit 1 while the default
// stays 0 and the human text does not change.
func TestDenyWarnings(t *testing.T) {
	dir := module(t, "main.kg", "let name = \"x\"\nprint \"hello $name\"\n")
	code, _, errOut := execute(t, "check", dir)
	if code != 0 || !strings.Contains(errOut, "warning:") {
		t.Fatalf("default: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = execute(t, "check", "--deny-warnings", dir)
	if code != 1 || !strings.Contains(errOut, "warning:") {
		t.Fatalf("--deny-warnings: exit %d, stderr %q", code, errOut)
	}
	code, out, _ := execute(t, "check", "--format", "json", dir)
	if code != 0 {
		t.Fatalf("json without denial: exit %d", code)
	}
	var rep diag.MachineReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Diagnostics) != 1 || rep.Diagnostics[0].Severity != "warning" {
		t.Errorf("diagnostics = %+v", rep.Diagnostics)
	}
}

// The allow list silences a code in both representations and lets the command
// exit 0; an unrelated code leaves the error in place.
func TestAllowList(t *testing.T) {
	dir := module(t, "main.kg", typeError)
	code, out, errOut := execute(t, "check", "--allow", "E400", dir)
	if code != 0 || out != "" || errOut != "" {
		t.Fatalf("allowed error: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	code, _, errOut = execute(t, "check", "--allow", "E999", dir)
	if code != 1 || !strings.Contains(errOut, "error[E400]") {
		t.Fatalf("unrelated allow: exit %d, stderr %q", code, errOut)
	}
	code, out, _ = execute(t, "check", "--format", "json", "--allow", "E400", dir)
	if code != 0 || !strings.Contains(out, "\"diagnostics\": []") {
		t.Fatalf("allowed json: exit %d, stdout %q", code, out)
	}
}

// --write-baseline records the current diagnostics, and --baseline silences
// exactly those: a moved diagnostic stays silent, a new one reports.
func TestBaseline(t *testing.T) {
	dir := module(t, "main.kg", typeError)
	base := filepath.Join(t.TempDir(), "diagnostics.json")
	code, _, errOut := execute(t, "check", "--write-baseline", base, dir)
	if code != 0 {
		t.Fatalf("--write-baseline: exit %d, stderr %q", code, errOut)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("baseline not written: %v", err)
	}
	code, _, errOut = execute(t, "check", "--baseline", base, dir)
	if code != 0 || errOut != "" {
		t.Fatalf("baselined: exit %d, stderr %q", code, errOut)
	}
	// The same diagnostics after an unrelated line shift stay baselined.
	if err := os.WriteFile(filepath.Join(dir, "main.kg"), []byte("print \"shift\"\n"+typeError), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = execute(t, "check", "--baseline", base, dir)
	if code != 0 || errOut != "" {
		t.Fatalf("shifted baselined: exit %d, stderr %q", code, errOut)
	}
	// A different message is a different fingerprint.
	if err := os.WriteFile(filepath.Join(dir, "main.kg"), []byte("let x: Int = \"s\"\nprint x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = execute(t, "check", "--baseline", base, dir)
	if code != 1 || !strings.Contains(errOut, "error[") {
		t.Fatalf("new diagnostic: exit %d, stderr %q", code, errOut)
	}
}

// An unknown --format and an unreadable --baseline are rejected values (exit
// 2), not command failures.
func TestDiagnosticFlagValues(t *testing.T) {
	dir := module(t, "main.kg", typeError)
	code, _, errOut := execute(t, "check", "--format", "yaml", dir)
	if code != exit.Value || !strings.Contains(errOut, "unknown --format") {
		t.Fatalf("bad format: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = execute(t, "check", "--baseline", filepath.Join(t.TempDir(), "missing.json"), dir)
	if code != exit.Value || !strings.Contains(errOut, "baseline") {
		t.Fatalf("missing baseline: exit %d, stderr %q", code, errOut)
	}
	code, _, errOut = execute(t, "check", "--baseline", "b.json", "--write-baseline", "c.json", dir)
	if code != exit.Usage || !strings.Contains(errOut, "mutually exclusive") {
		t.Fatalf("baseline and write-baseline: exit %d, stderr %q", code, errOut)
	}
}

// Default human output is unchanged: the machine flags are opt-in.
func TestHumanOutputUnchanged(t *testing.T) {
	dir := module(t, "main.kg", typeError)
	code, out, errOut := execute(t, "check", dir)
	if code != 1 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	for _, want := range []string{"error[E400]: expected `String`, found `Int`", " --> main.kg:1:", "^"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("human stderr missing %q:\n%s", want, errOut)
		}
	}
}

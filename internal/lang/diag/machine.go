package diag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// SchemaVersion versions the machine-readable diagnostic document check, run
// and build print under --format json. A consumer may rely on every field of
// a version it knows; adding a field does not change the version, removing or
// retyping one does.
const SchemaVersion = 1

// ToolName is the tool field of every machine report.
const ToolName = "kigumi"

// MachineReport is the versioned JSON envelope shared by check, run and build.
type MachineReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Tool          string              `json:"tool"`
	Diagnostics   []MachineDiagnostic `json:"diagnostics"`
}

// MachineDiagnostic is one diagnostic in the machine report. File and Span are
// absent for a diagnostic that has no source location (a release-policy
// finding, for instance).
type MachineDiagnostic struct {
	Severity    string        `json:"severity"`
	Code        string        `json:"code,omitempty"`
	Message     string        `json:"message"`
	Fingerprint string        `json:"fingerprint"`
	File        string        `json:"file,omitempty"`
	Span        *MachineSpan  `json:"span,omitempty"`
	Notes       []MachineNote `json:"notes,omitempty"`
	Help        []string      `json:"help,omitempty"`
}

// MachineNote is a note attached to a diagnostic.
type MachineNote struct {
	Severity string       `json:"severity"`
	Code     string       `json:"code,omitempty"`
	Message  string       `json:"message"`
	File     string       `json:"file,omitempty"`
	Span     *MachineSpan `json:"span,omitempty"`
}

// MachineSpan is a half-open byte range: Start is the first byte and End the
// byte after the last, so a zero-width insertion has equal positions.
type MachineSpan struct {
	Start MachinePosition `json:"start"`
	End   MachinePosition `json:"end"`
}

// MachinePosition is a 1-based line and byte column with the 0-based byte
// offset, so a consumer never has to re-scan the source for either.
type MachinePosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Offset int `json:"offset"`
}

// Entries maps diagnostics to their machine form, resolving each location
// through src. Diagnostics without a resolvable file keep an empty File and a
// nil Span.
func Entries(src Sources, ds []Diagnostic) []MachineDiagnostic {
	out := make([]MachineDiagnostic, 0, len(ds))
	for _, d := range ds {
		out = append(out, Entry(src, d))
	}
	return out
}

// Entry maps one diagnostic to its machine form.
func Entry(src Sources, d Diagnostic) MachineDiagnostic {
	e := MachineDiagnostic{Severity: d.Severity.String(), Code: d.Code, Message: d.Msg}
	e.File, e.Span = locate(src, d.Loc)
	e.Fingerprint = Fingerprint(e.Code, e.File, e.Message)
	for _, n := range d.Notes {
		ne := MachineNote{Severity: n.Severity.String(), Code: n.Code, Message: n.Msg}
		ne.File, ne.Span = locate(src, n.Loc)
		e.Notes = append(e.Notes, ne)
	}
	if len(d.Help) > 0 {
		e.Help = append([]string{}, d.Help...)
	}
	return e
}

// Fingerprint identifies a diagnostic across runs and line shifts: the code,
// file and message, never a position. A baseline stores fingerprints, so
// moving a baselined diagnostic does not resurrect it.
func Fingerprint(code, file, message string) string {
	sum := sha256.Sum256([]byte(code + "\x00" + file + "\x00" + message))
	return hex.EncodeToString(sum[:])
}

// Key is the fingerprint a baseline matches this entry by; an entry written by
// hand without one gets it computed from its code, file and message.
func (d MachineDiagnostic) Key() string {
	if d.Fingerprint != "" {
		return d.Fingerprint
	}
	return Fingerprint(d.Code, d.File, d.Message)
}

// Report assembles the machine report for ds.
func Report(src Sources, ds []Diagnostic) MachineReport {
	return MachineReport{SchemaVersion: SchemaVersion, Tool: ToolName, Diagnostics: Entries(src, ds)}
}

// Filter silences diagnostics a command was told to accept: by code (the allow
// list) or by baseline fingerprint.
type Filter struct {
	Allow    map[string]bool
	Baseline map[string]bool
}

// NewFilter builds a filter from an allow list of codes and a baseline set of
// fingerprints; nil or empty inputs silence nothing.
func NewFilter(allow []string, baseline map[string]bool) Filter {
	f := Filter{Baseline: baseline}
	if len(allow) > 0 {
		f.Allow = make(map[string]bool, len(allow))
		for _, code := range allow {
			if code != "" {
				f.Allow[code] = true
			}
		}
	}
	return f
}

// Drop reports whether the filter silences e.
func (f Filter) Drop(e MachineDiagnostic) bool {
	if e.Code != "" && f.Allow[e.Code] {
		return true
	}
	return len(f.Baseline) > 0 && f.Baseline[e.Key()]
}

// ReadBaseline loads the fingerprints of a report written by WriteBaseline.
func ReadBaseline(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read baseline %s: %w", path, err)
	}
	var rep MachineReport
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, fmt.Errorf("read baseline %s: %w", path, err)
	}
	if rep.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("read baseline %s: schemaVersion %d is newer than %d", path, rep.SchemaVersion, SchemaVersion)
	}
	out := make(map[string]bool, len(rep.Diagnostics))
	for _, d := range rep.Diagnostics {
		out[d.Key()] = true
	}
	return out, nil
}

// WriteBaseline saves rep as the baseline a later --baseline run silences.
func WriteBaseline(path string, rep MachineReport) error {
	if rep.Diagnostics == nil {
		rep.Diagnostics = []MachineDiagnostic{}
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write baseline %s: %w", path, err)
	}
	return nil
}

// Encode writes v as indented JSON followed by one newline, the framing every
// machine report and SARIF log uses. HTML escaping stays off: a diagnostic
// message reads as the compiler wrote it.
func Encode(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func locate(src Sources, loc Location) (string, *MachineSpan) {
	if src == nil {
		return "", nil
	}
	f := src.Source(loc.Source)
	if f == nil {
		return "", nil
	}
	span := MachineSpan{
		Start: MachinePosition{Line: f.Line(loc.Span.Start), Column: f.Column(loc.Span.Start), Offset: int(loc.Span.Start)},
		End:   MachinePosition{Line: f.Line(loc.Span.End), Column: f.Column(loc.Span.End), Offset: int(loc.Span.End)},
	}
	return f.Name, &span
}

// SARIF levels, in SARIF 2.1.0's spelling.
const (
	sarifError   = "error"
	sarifWarning = "warning"
	sarifNote    = "note"
)

// SARIFLog is the subset of SARIF 2.1.0 a diagnostics run needs.
type SARIFLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun is one tool invocation's results.
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool names the producing tool and declares its rules.
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver is the tool component's driver.
type SARIFDriver struct {
	Name  string      `json:"name"`
	Rules []SARIFRule `json:"rules,omitempty"`
}

// SARIFRule describes one diagnostic code.
type SARIFRule struct {
	ID               string    `json:"id"`
	ShortDescription SARIFText `json:"shortDescription"`
}

// SARIFResult is one diagnostic.
type SARIFResult struct {
	RuleID    string          `json:"ruleId,omitempty"`
	Level     string          `json:"level"`
	Message   SARIFText       `json:"message"`
	Locations []SARIFLocation `json:"locations,omitempty"`
}

// SARIFText is SARIF's message object.
type SARIFText struct {
	Text string `json:"text"`
}

// SARIFLocation points a result at a source region.
type SARIFLocation struct {
	PhysicalLocation SARIFPhysical `json:"physicalLocation"`
}

// SARIFPhysical is the artifact and region a location names.
type SARIFPhysical struct {
	ArtifactLocation SARIFArtifact `json:"artifactLocation"`
	Region           SARIFRegion   `json:"region"`
}

// SARIFArtifact is a file reference.
type SARIFArtifact struct {
	URI string `json:"uri"`
}

// SARIFRegion is a 1-based, half-open source range.
type SARIFRegion struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
	EndLine     int `json:"endLine"`
	EndColumn   int `json:"endColumn"`
}

// SARIF maps a machine report to a SARIF 2.1.0 log. Rules are sorted by code so
// the log is stable; a diagnostic without a code keeps its message and level
// and only loses ruleId.
func SARIF(rep MachineReport) SARIFLog {
	seen := map[string]bool{}
	var rules []SARIFRule
	results := make([]SARIFResult, 0, len(rep.Diagnostics))
	for _, d := range rep.Diagnostics {
		r := SARIFResult{RuleID: d.Code, Level: sarifLevel(d.Severity), Message: SARIFText{Text: d.Message}}
		if d.Code != "" && !seen[d.Code] {
			seen[d.Code] = true
			rules = append(rules, SARIFRule{ID: d.Code, ShortDescription: SARIFText{Text: "Kigumi diagnostic " + d.Code}})
		}
		if d.File != "" && d.Span != nil {
			r.Locations = []SARIFLocation{{PhysicalLocation: SARIFPhysical{
				ArtifactLocation: SARIFArtifact{URI: d.File},
				Region:           sarifRegion(*d.Span),
			}}}
		}
		results = append(results, r)
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return SARIFLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []SARIFRun{{
			Tool:    SARIFTool{Driver: SARIFDriver{Name: rep.Tool, Rules: rules}},
			Results: results,
		}},
	}
}

func sarifLevel(severity string) string {
	switch severity {
	case "error":
		return sarifError
	case "warning":
		return sarifWarning
	}
	return sarifNote
}

// sarifRegion converts a half-open span to SARIF's 1-based half-open region. A
// zero-width span keeps one column so SARIF readers still underline a point.
func sarifRegion(s MachineSpan) SARIFRegion {
	endLine, endColumn := s.End.Line, s.End.Column
	if s.End.Offset == s.Start.Offset {
		endLine, endColumn = s.Start.Line, s.Start.Column+1
	}
	return SARIFRegion{StartLine: s.Start.Line, StartColumn: s.Start.Column, EndLine: endLine, EndColumn: endColumn}
}

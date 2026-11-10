package session

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"kigumi/internal/base/errors"
	"kigumi/internal/cli/exit"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/policy"
)

// DiagOptions is the diagnostic-reporting flag set check, run and build share:
// which representation the diagnostics take, which ones the reader has already
// accepted, and whether a leftover warning must fail the command.
type DiagOptions struct {
	Format        string
	DenyWarnings  bool
	Allow         []string
	Baseline      string
	WriteBaseline string
}

// AddDiagFlags registers the shared diagnostic flags on cmd.
func AddDiagFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.String("format", "human", "diagnostics output: human, json (the versioned machine schema) or sarif")
	f.Bool("deny-warnings", false, "exit 1 when a warning survives the allow list and the baseline")
	f.StringArray("allow", nil, "diagnostic code to silence, repeatable or comma-separated (for example E401,E402)")
	f.String("baseline", "", "JSON diagnostic report written by --write-baseline; its entries are silenced on this run")
	f.String("write-baseline", "", "write this run's diagnostics to a JSON report and exit 0")
}

// ReadDiagOptions reads the shared flags from cmd.
func ReadDiagOptions(cmd *cobra.Command) (DiagOptions, error) {
	f := cmd.Flags()
	var o DiagOptions
	o.Format, _ = f.GetString("format")
	switch o.Format {
	case "human", "json", "sarif":
	default:
		return o, &exit.ValueError{Err: errors.NewErrf("unknown --format %q: want human, json or sarif", o.Format)}
	}
	o.DenyWarnings, _ = f.GetBool("deny-warnings")
	allow, _ := f.GetStringArray("allow")
	o.Allow = splitCodes(allow)
	o.Baseline, _ = f.GetString("baseline")
	o.WriteBaseline, _ = f.GetString("write-baseline")
	if o.Baseline != "" && o.WriteBaseline != "" {
		return o, &exit.UsageError{Err: errors.NewErr("--baseline and --write-baseline are mutually exclusive")}
	}
	return o, nil
}

// Machine reports whether the options ask for a machine-readable document.
func (o DiagOptions) Machine() bool { return o.Format == "json" || o.Format == "sarif" }

// Filtering reports whether the options silence or deny diagnostics, so a
// command has to look at their structure rather than their text.
func (o DiagOptions) Filtering() bool {
	return o.Machine() || o.DenyWarnings || len(o.Allow) > 0 || o.Baseline != "" || o.WriteBaseline != ""
}

// Report accumulates what a command reports, writes the human text as it
// arrives, and renders the machine document once at the end.
type Report struct {
	opts    DiagOptions
	filter  diag.Filter
	human   io.Writer
	entries []diag.MachineDiagnostic
	errs    int
	warns   int
}

// NewReport builds a report; a missing or unreadable --baseline is a rejected
// value, not an internal failure.
func NewReport(opts DiagOptions, human io.Writer) (*Report, error) {
	r := &Report{opts: opts, human: human, entries: []diag.MachineDiagnostic{}}
	var baseline map[string]bool
	if opts.Baseline != "" {
		b, err := diag.ReadBaseline(opts.Baseline)
		if err != nil {
			return nil, &exit.ValueError{Err: err}
		}
		baseline = b
	}
	r.filter = diag.NewFilter(opts.Allow, baseline)
	return r, nil
}

// Human reports whether human text is written as diagnostics arrive.
func (r *Report) Human() bool { return !r.opts.Machine() }

// Add keeps the diagnostics the allow list and the baseline do not silence,
// writes them human-style, and records them for the machine document.
func (r *Report) Add(src diag.Sources, ds []diag.Diagnostic) {
	for _, d := range ds {
		if !r.add(diag.Entry(src, d)) {
			continue
		}
		if r.Human() && r.human != nil {
			io.WriteString(r.human, diag.Render(src, d))
		}
	}
}

// AddMachine records diagnostics for the machine document without writing
// their human text; a caller that rendered its own block uses this.
func (r *Report) AddMachine(src diag.Sources, ds []diag.Diagnostic) {
	for _, d := range ds {
		r.add(diag.Entry(src, d))
	}
}

// AddPolicy reports a release-policy result: human text is the policy report's
// own layout, the machine document sees the same findings as diagnostics with
// their stable ID as the code and no source location.
func (r *Report) AddPolicy(rep policy.Report) {
	kept := policy.Report{}
	for _, v := range rep.Warnings {
		if r.keepCode(v.ID, v.Subject) {
			kept.Warnings = append(kept.Warnings, v)
		}
	}
	for _, v := range rep.Errors {
		if r.keepCode(v.ID, v.Subject) {
			kept.Errors = append(kept.Errors, v)
		}
	}
	if r.Human() && r.human != nil {
		io.WriteString(r.human, kept.Render())
	}
	for _, v := range kept.Warnings {
		r.add(policyEntry(diag.Warning, v))
	}
	for _, v := range kept.Errors {
		r.add(policyEntry(diag.Error, v))
	}
}

// Text writes an already-rendered human block; the machine document is
// unaffected.
func (r *Report) Text(text string) {
	if r.Human() && r.human != nil {
		io.WriteString(r.human, text)
	}
}

// Emit writes the machine document when --format chose one.
func (r *Report) Emit(w io.Writer) error {
	switch r.opts.Format {
	case "json":
		return diag.Encode(w, r.report())
	case "sarif":
		return diag.Encode(w, diag.SARIF(r.report()))
	}
	return nil
}

// WriteBaseline saves this run's diagnostics under --write-baseline.
func (r *Report) WriteBaseline() error {
	if r.opts.WriteBaseline == "" {
		return nil
	}
	return diag.WriteBaseline(r.opts.WriteBaseline, r.report())
}

// WritingBaseline reports whether --write-baseline asked for a file.
func (r *Report) WritingBaseline() bool { return r.opts.WriteBaseline != "" }

// HasErrors reports whether an error survived the allow list and baseline.
func (r *Report) HasErrors() bool { return r.errs > 0 }

// Failed reports whether the kept diagnostics fail the command: any error, or
// any warning under --deny-warnings.
func (r *Report) Failed() bool {
	return r.errs > 0 || (r.opts.DenyWarnings && r.warns > 0)
}

func (r *Report) report() diag.MachineReport {
	return diag.MachineReport{SchemaVersion: diag.SchemaVersion, Tool: diag.ToolName, Diagnostics: r.entries}
}

func (r *Report) count(sev diag.Severity) {
	switch sev {
	case diag.Error:
		r.errs++
	case diag.Warning:
		r.warns++
	}
}

// add records e unless the filter silences it.
func (r *Report) add(e diag.MachineDiagnostic) bool {
	if r.filter.Drop(e) {
		return false
	}
	r.entries = append(r.entries, e)
	r.count(severityOf(e.Severity))
	return true
}

// keepCode reports whether a location-less diagnostic survives the filter.
func (r *Report) keepCode(code, message string) bool {
	return !r.filter.Drop(diag.Entry(nil, diag.Diagnostic{Code: code, Msg: message}))
}

func severityOf(e string) diag.Severity {
	switch e {
	case "error":
		return diag.Error
	case "warning":
		return diag.Warning
	}
	return diag.Note
}

// policyEntry turns one policy finding into the machine document's entry: the
// stable ID as the code, the subject as the message and the detail as notes.
func policyEntry(sev diag.Severity, v policy.Violation) diag.MachineDiagnostic {
	d := diag.Diagnostic{Severity: sev, Code: v.ID, Msg: v.Subject}
	for _, kv := range v.Detail {
		d.Notes = append(d.Notes, diag.Diagnostic{Severity: diag.Note, Msg: kv.Key + ": " + kv.Value})
	}
	if v.Policy != "" {
		d.Notes = append(d.Notes, diag.Diagnostic{Severity: diag.Note, Msg: "release policy: " + v.Policy})
	}
	if v.Help != "" {
		d.Help = append(d.Help, v.Help)
	}
	return diag.Entry(nil, d)
}

// splitCodes trims a repeatable flag's values and splits comma-separated ones.
func splitCodes(values []string) []string {
	var out []string
	for _, v := range values {
		for _, code := range strings.Split(v, ",") {
			if code = strings.TrimSpace(code); code != "" {
				out = append(out, code)
			}
		}
	}
	return out
}

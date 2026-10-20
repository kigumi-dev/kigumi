package testtool

import "encoding/json"

// Audit is the B0 test-inventory snapshot uploaded as a CI artifact.
type Audit struct {
	Packages  []PackageCounts `json:"packages"`
	Fixtures  FixtureCounts   `json:"fixtures"`
	Toolchain Toolchain       `json:"toolchain"`
	Totals    AuditTotals     `json:"totals"`
}

// AuditTotals sums Packages across the whole repository.
type AuditTotals struct {
	Tests          int `json:"tests"`
	Benchmarks     int `json:"benchmarks"`
	Fuzz           int `json:"fuzz"`
	Skips          int `json:"skips"`
	PackagesWithGo int `json:"packagesWithGo"`
	UntestedGo     int `json:"untestedGoPackages"`
}

// BuildAudit runs every counter in this package and assembles the report.
func BuildAudit(repoRoot string, look Lookup) (*Audit, error) {
	pkgs, err := CountGoTests(repoRoot, "testdata", "node_modules", ".git")
	if err != nil {
		return nil, err
	}
	fixtures, err := CountFixtures(repoRoot)
	if err != nil {
		return nil, err
	}
	a := &Audit{Packages: pkgs, Fixtures: fixtures, Toolchain: DiscoverToolchain(look)}
	for _, p := range pkgs {
		a.Totals.Tests += p.Tests
		a.Totals.Benchmarks += p.Benchmarks
		a.Totals.Fuzz += p.Fuzz
		a.Totals.Skips += p.Skips
		a.Totals.PackagesWithGo++
		if !p.HasTests {
			a.Totals.UntestedGo++
		}
	}
	return a, nil
}

// JSON renders the audit as indented JSON with a trailing newline.
func (a *Audit) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

package driver

// UnitStats counts what CheckUnits did per package.
type UnitStats struct {
	Runs int
	// Checked counts the runs a package was checked as a unit of its own.
	Checked map[string]int
	// Context counts the runs that loaded a package only because a checked unit imports it.
	Context map[string]int
	// Replayed counts the packages skipped on a clean record.
	Replayed map[string]int
	// Demoted counts the clean records withdrawn because the package reported a diagnostic.
	Demoted int
	// Fallbacks counts the checks that gave up on units and ran the whole module.
	Fallbacks int
}

// UnitStats reports what CheckUnits did for this module.
func (m *Module) UnitStats() UnitStats { return m.unitStats }

func (s *UnitStats) bump(field *map[string]int, path string) {
	if *field == nil {
		*field = map[string]int{}
	}
	(*field)[path]++
}

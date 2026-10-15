package driver_test

import (
	"sync"
	"testing"

	"kigumi/internal/driver"
)

// fixtureCheck is what the VM-coverage and manifest tests both need from one run fixture.
type fixtureCheck struct {
	done     bool
	err      error
	errs     string
	refusal  error
	wantErr  string
	wantExit int
}

type fixtureCheckEntry struct {
	once sync.Once
	v    fixtureCheck
}

var fixtureChecks sync.Map

// checkedFixture analyzes a run fixture once per test binary, whichever test asks first.
func checkedFixture(t testing.TB, path string) fixtureCheck {
	t.Helper()
	e, _ := fixtureChecks.LoadOrStore(path, &fixtureCheckEntry{})
	ent := e.(*fixtureCheckEntry)
	ent.once.Do(func() { ent.v = analyzeRunFixture(t, path) })
	if ent.v.err != nil {
		t.Fatal(ent.v.err)
	}
	if !ent.v.done {
		t.Fatalf("%s: analysis did not finish", path)
	}
	return ent.v
}

func analyzeRunFixture(t testing.TB, path string) fixtureCheck {
	root, _, wantErr, wantExit := loadRunFixture(t, path)
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
	if err != nil {
		return fixtureCheck{err: err}
	}
	a, err := driver.Analyze(m)
	if err != nil {
		return fixtureCheck{err: err}
	}
	v := fixtureCheck{done: true, wantErr: wantErr, wantExit: wantExit}
	if a.Res.HasErrors() {
		v.errs = a.Res.Render()
		return v
	}
	v.refusal = driver.VMRefusal(a)
	return v
}

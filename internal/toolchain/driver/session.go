package driver

import (
	"fmt"
	"io"
	"sync"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// CompilationSession shares one parse snapshot and one lowered analysis across
// Check, Run and Build of the same module bytes. A caller creates one session
// per editing/compilation episode:
//
//	s := driver.NewCompilationSession()
//	m.AttachSession(s)
//	driver.Check(m)     // checks and lowers once
//	driver.Run(m, ...)  // reuses that analysis
//
// The key is the content-derived ModuleID, so a source change invalidates the
// cached analysis without anyone remembering to clear it.
type CompilationSession struct {
	mu       sync.Mutex
	parse    *syntax.ParseSnapshot
	analysis *Analysis
	key      string
	busy     bool
}

// NewCompilationSession returns an empty session.
func NewCompilationSession() *CompilationSession { return &CompilationSession{} }

// AttachSession makes m use s for its analyses. It does not analyze anything
// by itself.
func (m *Module) AttachSession(s *CompilationSession) { m.session = s }

// Session returns m's attached session, creating one on first use.
func (m *Module) Session() *CompilationSession {
	if m.session == nil {
		m.session = NewCompilationSession()
	}
	return m.session
}

// ParseSnapshot returns m's parse snapshot, reusing the one already computed
// for the same content.
func (s *CompilationSession) ParseSnapshot(m *Module) *syntax.ParseSnapshot {
	snap := m.ParseSnapshot()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.parse != nil && s.parse.ModuleID() == snap.ModuleID() {
		return s.parse
	}
	s.parse = snap
	return s.parse
}

// Analyze returns a shared analysis, lowering once per ModuleID.
func (s *CompilationSession) Analyze(m *Module) (*Analysis, error) {
	m.AttachSession(s)
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return Analyze(m)
	}
	s.busy = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()
	a, err := Analyze(m)
	if err != nil || a == nil || !cacheableAnalysis(a) {
		return a, err
	}
	key := m.analysisKey()
	s.mu.Lock()
	s.analysis, s.key = a, key
	s.mu.Unlock()
	return a, nil
}

// Snapshot returns the published immutable view shared by the session; nil
// before the first analysis.
func (s *CompilationSession) Snapshot() *sem.AnalysisSnapshot {
	s.mu.Lock()
	a := s.analysis
	s.mu.Unlock()
	if a == nil {
		return nil
	}
	return a.Snapshot()
}

// Check is driver.Check sharing this session.
func (s *CompilationSession) Check(m *Module) (*sem.Result, error) {
	m.AttachSession(s)
	return Check(m)
}

// Run is driver.RunWith sharing this session.
func (s *CompilationSession) Run(m *Module, opts RunOptions, stdout, stderr io.Writer, args []string) (int, error) {
	m.AttachSession(s)
	return RunWith(m, opts, stdout, stderr, args)
}

// BuildWith is driver.BuildWith sharing this session.
func (s *CompilationSession) BuildWith(m *Module, out string, stderr io.Writer, opts BuildOptions) (bool, error) {
	m.AttachSession(s)
	return BuildWith(m, out, stderr, opts)
}

// Invalidate forgets the cached analysis; the next Analyze recomputes it.
func (s *CompilationSession) Invalidate() {
	s.mu.Lock()
	s.analysis, s.key = nil, ""
	s.mu.Unlock()
}

// reuseAnalysis returns m's cached analysis when it is still current. It is
// the hook AnalyzeWith consults, so Check/Run/Build on an attached module
// share one lowering.
func (m *Module) reuseAnalysis() (*Analysis, bool) {
	s := m.session
	if s == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy || s.analysis == nil || s.key != m.analysisKey() {
		return nil, false
	}
	return s.analysis, true
}

// rememberAnalysis records a completed analysis for the attached session.
func (m *Module) rememberAnalysis(a *Analysis) {
	s := m.session
	if s == nil || a == nil || !cacheableAnalysis(a) {
		return
	}
	s.mu.Lock()
	s.analysis, s.key = a, m.analysisKey()
	s.mu.Unlock()
}

// analysisKey names the compilation a cached analysis answers for: the parsed
// content plus the options that change what a check or lowering produces.
func (m *Module) analysisKey() string {
	return fmt.Sprintf("%s|%s|%+v|%t|%t|%t|%t",
		m.ParseSnapshot().ModuleID(), m.Entry, m.target, m.test, m.lowerAll, m.verifyPasses, m.lazyStd)
}

// cacheableAnalysis is false for a run that skipped bodies: a later consumer
// may need one, and the cache must not answer for it.
func cacheableAnalysis(a *Analysis) bool {
	return a.Res != nil && a.Res.SkippedBodies() == 0 && (a.Prog == nil || len(a.Prog.Skipped) == 0)
}

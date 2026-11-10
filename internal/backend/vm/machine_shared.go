package vm

import (
	"bufio"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"syscall"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/buildgraph"
)

// shared is the machine state more than one lane can reach: every field below
// is guarded by one of these mutexes or atomic. The rest lives on the Machine.
type shared struct {
	// sched covers the scheduler: both queues, task fields, executor counters.
	sched    sync.Mutex
	orphans  []*executor
	closeGen uint64
	pools    []*executor
	// out covers stdout and stderr, in the single stdin reader; they are
	// separate so a lane parked on a read does not hold the printers.
	out      sync.Mutex
	in       sync.Mutex
	stdin    *bufio.Reader
	stdinSrc io.Reader
	// cwd is the directory the program's relative paths resolve against. It
	// starts at Options.Cwd (the process directory when empty) and Host.chdir
	// moves it; every lane sees the move.
	cwdMu sync.RWMutex
	cwd   string
	// writeFail is the first print/eprint that could not be written, reported once when the entry returns.
	writeFail *writeFailure
	// derived covers the tables the lanes fill in lazily from the program.
	derived    sync.Mutex
	codes      sync.Map
	drops      map[sem.EntityID]*midir.Func
	named      map[string]sem.EntityID
	borrowMask map[sem.EntityID]uint64
	underlying map[sem.TypeID]sem.TypeID
	// store covers the tables a program's own values index into; what one entry
	// holds belongs to a move-only resource, so only the table itself is shared.
	store        sync.Mutex
	globals      map[sem.EntityID]*obj
	mmio         map[uint64]*obj
	cvalues      []*obj
	handleTables []*handleTableState
	sigWatch     map[string]*sigWatch
	// procMu guards procs, the live OS processes std/proc spawned.
	procMu sync.Mutex
	procs  map[int64]*vmProcess
	// shadows is SecretSan's value-label table (nil unless Options.SecretSan).
	// A value can migrate to another lane with a task, so the table is shared.
	shadowMu   sync.Mutex
	shadows    map[*obj]label
	mem        atomic.Int64
	faultN     atomic.Int64
	faultArmed atomic.Bool
	// steps is shared by all lanes so a thread pool has one StepLimit.
	steps atomic.Int64
	// cancel asks the instruction loops to abort; a session sets it from
	// another goroutine and clears it when the interrupted cell settles.
	cancel atomic.Bool
	// effectsOff refuses host effects: a test whose timeout already stands may still be unwinding, and an effect it
	// has not begun yet would land after the report.
	effectsOff atomic.Bool
	// drainLeft is the instruction budget a timed-out test may still run: enough for the statement it is in to
	// finish (so its output and panic are reportable), not enough for a runaway loop to outlive the report.
	drainLeft atomic.Int64
	// graph is written before the run starts and only read after.
	graph *buildgraph.Graph
}

func newShared() *shared {
	return &shared{
		drops:      map[sem.EntityID]*midir.Func{},
		named:      map[string]sem.EntityID{},
		borrowMask: map[sem.EntityID]uint64{},
		underlying: map[sem.TypeID]sem.TypeID{},
		globals:    map[sem.EntityID]*obj{},
	}
}

func (m *Machine) lane() *Machine {
	w := *m
	w.stack, w.pending, w.spare, w.pool, w.slab, w.frames = nil, nil, nil, nil, nil, nil
	w.stepsLeft, w.depth = 0, 0
	w.site, w.siteFn, w.curFn, w.curInst, w.valueSites, w.boxFacts, w.boxParams = nil, nil, nil, nil, nil, nil, nil
	w.cur, w.curLane = nil, 0
	return &w
}

// Resolving a defined type can intern a new one in the checker's table, so
// the lock covers the call and not just the memo.
func (m *Machine) underlyingRepr(t sem.TypeID) sem.TypeID {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	if u, ok := m.sh.underlying[t]; ok {
		return u
	}
	u := m.r.UnderlyingRepr(t)
	m.sh.underlying[t] = u
	return u
}

func (m *Machine) writeOut(b []byte) (int, error) {
	m.sh.out.Lock()
	defer m.sh.out.Unlock()
	// The runtime turns a closed fd 1 into /dev/null before main, so this is the only place the loss can be noticed.
	if f, ok := m.out.(*os.File); ok && f == os.Stdout && stdoutWasClosed() {
		return 0, syscall.EBADF
	}
	return m.out.Write(b)
}

func (m *Machine) writeErr(b []byte) (int, error) {
	m.sh.out.Lock()
	defer m.sh.out.Unlock()
	return m.err.Write(b)
}

func (m *Machine) addCValue(v *obj) int64 {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	m.sh.cvalues = append(m.sh.cvalues, v)
	return int64(len(m.sh.cvalues))
}

func (m *Machine) cvalue(hd int64) *obj {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	return m.sh.cvalues[hd-1]
}

func (m *Machine) replaceCValue(hd int64, v *obj) {
	m.sh.store.Lock()
	old := m.sh.cvalues[hd-1]
	m.sh.cvalues[hd-1] = v
	m.sh.store.Unlock()
	m.release(old)
}

func (m *Machine) addHandleTable() int64 {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	m.sh.handleTables = append(m.sh.handleTables, &handleTableState{})
	return int64(len(m.sh.handleTables))
}

func (m *Machine) handleTable(i int64) *handleTableState {
	m.sh.store.Lock()
	defer m.sh.store.Unlock()
	return m.sh.handleTables[i-1]
}

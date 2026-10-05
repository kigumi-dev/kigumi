package vm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	"kigumi/internal/buildgraph"
	"kigumi/internal/diag"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// Panic is a program abort; Site is the panicking instruction's user frame, zero when none (e.g. out of memory).
type Panic struct {
	Msg  string
	Site mir.Site
}

func (p *Panic) Error() string { return "panic: " + p.Msg }

// Unsupported reports an instruction or primitive the VM does not run yet.
type Unsupported struct{ What string }

func (u *Unsupported) Error() string { return "vm: " + u.What + " is not supported" }

// Options is what the host hands a run: the program arguments, stdin,
// the directory relative paths resolve against, and whether the Kigumi
// bodies of host-backed std functions run instead of the accelerators.
type Options struct {
	Args    []string
	Stdin   io.Reader
	Cwd     string
	NoAccel bool
	// Sandbox refuses host effects and bounds the run by StepLimit
	// instructions, DepthLimit nested calls and MemoryLimit allocated
	// bytes, so a comptime block or build program fails
	// with a diagnostic instead of hanging or exhausting RAM. MemoryLimit
	// of 0 leaves allocation unbounded (EvalBuild does not set it).
	Sandbox     bool
	StepLimit   int
	DepthLimit  int
	MemoryLimit int64
	// DenySignals refuses Signals.watch instead of installing a real OS
	// handler: `kigumi test` runs in the same process as the CLI, and a
	// real sigaction/os-signal.Notify there would hijack the runner's own
	// Ctrl-C handling for the rest of the invocation.
	DenySignals bool
	// FaultInjectAt mirrors rt_core.c's KIGUMI_FAIL_ALLOC_AT: the Nth
	// allocation attempt refuses instead of succeeding; 0 disables it.
	FaultInjectAt int64
}

// Machine runs one program.
type Machine struct {
	p        *mir.Program
	r        *sem.Result
	out, err io.Writer
	opts     Options
	drops    map[sem.EntityID]*mir.Func
	stdin    *bufio.Reader
	stdinSrc io.Reader
	named    map[string]sem.EntityID
	host     map[string]hostFn
	hostByOp map[mir.ProviderOpId]hostFn
	cvalues  []*obj
	// handleTables backs std/ffi's HandleTable[T]; see std_ffi_handle.go.
	handleTables []*handleTableState
	// globals is every owned/exported static's storage slot:
	// an immortal (rc -1) object, replaced wholesale on a `static mut` write.
	globals map[sem.EntityID]*obj
	// site is the call being dispatched, for primitives whose meaning
	// depends on the argument types at the call (ffi.sizeOf).
	site   *mir.Inst
	siteFn *mir.Func
	// curFn/curInst track every instruction (not just calls, unlike site/siteFn), for Panic's Site.
	curFn   *mir.Func
	curInst *mir.Inst
	// stack is the active calls, innermost last; std/build reads the
	// nearest non-std caller from it as a step's provenance.
	stack        []*mir.Func
	steps, depth int
	// cur is the running task and curLane its executor lane, for spawn and
	// cancellation; only the worker holding run may touch them.
	cur     *task
	curLane int
	// run serializes task bodies; sched covers the scheduler state a worker
	// touches around a poll, and pending holds releases it owes once it unlocks.
	run     sync.Mutex
	sched   sync.Mutex
	pending []*obj
	// orphans are dropped thread pools a worker had not left yet.
	orphans  []*executor
	mem      int64
	faultN   int64
	graph    *buildgraph.Graph
	sigWatch map[string]chan os.Signal
}

// Run executes the program's entry and returns its exit code: 0, 1 for a
// main that fails with an Error, 2 for a panic.
func Run(prog *mir.Program, opts Options, out, errOut io.Writer) (int, error) {
	if prog.Entry == nil {
		return 1, fmt.Errorf("vm: program has no entry")
	}
	m := newMachine(prog, opts, out, errOut)
	var code int
	err := m.catch(func() {
		res := m.call(prog.Entry, nil)
		code = m.exitCode(res)
	})
	if p, ok := err.(*Panic); ok {
		fmt.Fprintf(errOut, "%s: %s\n", diag.Prefix("panic"), p.Msg)
		return 2, nil
	}
	return code, err
}

func newMachine(prog *mir.Program, opts Options, out, errOut io.Writer) *Machine {
	m := &Machine{p: prog, r: prog.R, out: out, err: errOut, opts: opts, drops: map[sem.EntityID]*mir.Func{}, named: map[string]sem.EntityID{}}
	m.registerAll()
	m.initGlobals()
	return m
}

// registerAll installs every host accelerator: the Go implementations that
// stand in for std bodies the VM cannot run (extern(C) hosts) or has no
// body for.
func (m *Machine) registerAll() {
	m.registerHost()
	m.registerShell()
	m.registerTask()
	m.registerBlocking()
	m.registerFFI()
	m.registerFFIHandle()
	m.registerTime()
	m.registerSignal()
	m.registerBuild()
	m.registerNet()
	m.registerNetHTTP()
	m.registerEntropy()
	m.registerError()
	m.indexHostByOp()
}

// catch turns the machine's aborts (raised with panic) into errors.
func (m *Machine) catch(f func()) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			switch e := rec.(type) {
			case *Panic, *Unsupported:
				err = e.(error)
			default:
				panic(rec)
			}
		}
	}()
	f()
	return nil
}

func (m *Machine) abort(msg string) {
	var site mir.Site
	if m.curFn != nil && m.curInst != nil {
		site = m.p.SiteAt(m.curFn, m.curInst.Node)
	}
	panic(&Panic{Msg: msg, Site: site})
}

func (m *Machine) exitCode(res *obj) int {
	res = deref(res)
	if res != nil && res.k == kVariant && res.ent == m.variantOf(m.r.Types.ResultEnt(), "Err") {
		fmt.Fprintf(m.err, "%s: %s\n", diag.Prefix("error"), m.display(m.errorMessage(res.fields[0])))
		return 1
	}
	return 0
}

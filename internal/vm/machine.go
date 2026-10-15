package vm

import (
	"fmt"
	"io"

	"kigumi/internal/diag"
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// Panic is a program abort; Site is the panicking instruction's user frame, zero when none (e.g. out of memory).
type Panic struct {
	Msg  string
	Site mir.Site
	// Sandbox marks a budget abort, never contained as a worker's own fault.
	Sandbox bool
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
	// FaultArmOnEprint mirrors KIGUMI_FAIL_ALLOC_ARM: no tick counts until the first eprint returns.
	FaultArmOnEprint bool
}

// Machine is one lane of a run: the interpreter state below belongs to the
// goroutine holding it, and sh is what every lane shares.
type Machine struct {
	p        *mir.Program
	r        *sem.Result
	out, err io.Writer
	opts     Options
	host     map[string]hostFn
	hostByOp map[mir.ProviderOpId]hostFn
	sh       *shared
	// site is the call being dispatched, for primitives whose meaning
	// depends on the argument types at the call (ffi.sizeOf).
	site       *mir.Inst
	siteFn     *mir.Func
	valueSites map[valueSiteKey]valueSite
	// curFn/curInst track every instruction (not just calls, unlike site/siteFn), for Panic's Site.
	curFn   *mir.Func
	curInst *mir.Inst
	// stack is the active calls, innermost last; std/build reads the
	// nearest non-std caller from it as a step's provenance.
	stack []*mir.Func
	// frames stay on a panic, for the task boundary to sweep.
	frames           []*frame
	spare            []*frame
	pool             []*obj
	slab             []*obj
	sweepFault       *Panic
	outerDepth       int
	stepsLeft, depth int
	// cur is the running task and curLane its executor lane, for spawn and cancellation.
	cur     *task
	curLane int
	// pending holds releases this lane owes once it leaves the scheduler lock.
	pending []*obj
	// witnessed memoizes Supports' answer for a requirement and a method of its name.
	witnessed map[[2]sem.EntityID]bool
}

// Run executes the program's entry and returns its exit code: 0, 1 for a
// main that fails with an Error, 2 for a panic.
func Run(prog *mir.Program, opts Options, out, errOut io.Writer) (int, error) {
	if prog.Entry == nil {
		return 1, fmt.Errorf("vm: program has no entry")
	}
	return runMachine(newMachine(prog, opts, out, errOut), prog, errOut)
}

func runMachine(m *Machine, prog *mir.Program, errOut io.Writer) (int, error) {
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
	m := &Machine{p: prog, r: prog.R, out: out, err: errOut, opts: opts, sh: newShared()}
	if !opts.Sandbox {
		prog.ContainsDropPanics()
	}
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
	base := len(m.frames)
	defer func() {
		m.frames = m.frames[:min(base, len(m.frames))]
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
	panic(&Panic{Msg: msg, Site: m.abortSite()})
}

func (m *Machine) sandboxAbort(msg string) {
	panic(&Panic{Msg: msg, Site: m.abortSite(), Sandbox: true})
}

func (m *Machine) abortSite() mir.Site {
	if m.curFn != nil && m.curInst != nil {
		return m.p.SiteAt(m.curFn, m.curInst.Node)
	}
	return mir.Site{}
}

func (m *Machine) exitCode(res *obj) int {
	res = deref(res)
	if res != nil && res.k == kVariant && res.ent == m.variantOf(m.r.Types.ResultEnt(), "Err") {
		m.writeErr([]byte(fmt.Sprintf("%s: %s\n", diag.Prefix("error"), m.display(m.errorMessage(res.fields[0])))))
		return 1
	}
	return 0
}

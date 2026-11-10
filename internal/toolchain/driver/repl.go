package driver

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// ReplBinding is one value held by the host between cells. Type is the source
// spelling the checker gave the binding, so the next cell's wrapper can declare
// it without asking the checker twice.
type ReplBinding struct {
	Name  string
	Type  string
	Mut   bool
	Value vm.Value
}

// ReplOptions is the process context of one interactive session.
type ReplOptions struct {
	Load  LoadOptions
	VM    vm.Options
	Out   io.Writer
	Err   io.Writer
	Root  string
	Entry string
}

// Repl owns the append-only session source and the values that survive cells.
type Repl struct {
	root     string
	load     LoadOptions
	vmOpts   vm.Options
	out, err io.Writer
	entry    string
	decls    string
	bindings []ReplBinding
	seq      int
	shell    bool
	sess     *vm.Session
}

// EvalResult is the observable result of one cell. Message holds rendered
// diagnostics or a panic; Display is the interactive value, if any.
type EvalResult struct {
	Display    string
	Message    string
	Status     int
	Incomplete bool
	Exit       bool
	ExitCode   int
}

// NewRepl starts a session over root. An empty Entry chooses
// __kigumi_repl__.kg; a collision gets a numbered name.
func NewRepl(root string, opts ReplOptions) (*Repl, error) {
	if root == "" {
		root = opts.Root
	}
	if root == "" {
		root = opts.Load.Compiler.Cwd
	}
	if root == "" {
		return nil, fmt.Errorf("repl: module root is required")
	}
	cwd := opts.Load.Compiler.Cwd
	if !filepath.IsAbs(root) && cwd != "" {
		root = filepath.Join(cwd, root)
	}
	abs := root
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("repl: %s is not a directory", abs)
	}
	if opts.Load.StdRoot != "" && !filepath.IsAbs(opts.Load.StdRoot) && cwd != "" {
		opts.Load.StdRoot = filepath.Join(cwd, opts.Load.StdRoot)
	}
	opts.Load.Compiler.Cwd = abs
	if opts.VM.Cwd == "" {
		opts.VM.Cwd = abs
	}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	entry := opts.Entry
	if entry == "" {
		entry = "__kigumi_repl__.kg"
		for i := 1; ; i++ {
			if _, err := os.Stat(filepath.Join(abs, entry)); os.IsNotExist(err) {
				break
			}
			entry = fmt.Sprintf("__kigumi_repl_%d__.kg", i)
		}
	}
	if filepath.IsAbs(entry) || filepath.Dir(entry) != "." {
		return nil, fmt.Errorf("repl: entry %q must be a root-relative file name", entry)
	}
	opts.Entry = entry
	return &Repl{root: abs, load: opts.Load, vmOpts: opts.VM, out: opts.Out, err: opts.Err, entry: entry, decls: replPrelude}, nil
}

// replPrelude is the shell-independent part of the session scope. The shell
// helper is added only when a cell mentions a Plan or shell value, so a plain
// arithmetic session does not pull std/shell into every check.
const replPrelude = `import os from std/os

fn cd(path: String) -> Void! {
    host.chdir(path)?
    host.setEnv("PWD", host.cwd())?
}

fn pwd() -> String {
    host.cwd()
}

fn setEnv(name: String, value: String) -> Void! {
    host.setEnv(name, value)?
}

fn unsetEnv(name: String) -> Void! {
    host.unsetEnv(name)?
}

fn env(name: String) -> String? {
    host.env(name)
}

fn args() -> os.Args {
    host.args()
}

fn exit(code: u8) -> Never {
    host.exit(code)
}
`

const replShellPrelude = `import shell from std/shell

fn sh() -> shell.Shell {
    host.shell()
}

fn __repl_run_plan(plan: shell.Plan) -> Int! {
    let result = shell.runHere(plan, host.shell())?
    let mut status = 0
    for s in result.statuses {
        if s != 0 {
            status = s
        }
    }
    status
}
`

// Close releases the persistent VM session.
func (r *Repl) Close() {
	if r == nil {
		return
	}
	if r.sess != nil {
		r.sess.Close()
		r.sess = nil
	}
	for i := range r.bindings {
		r.bindings[i].Value.Release()
	}
	r.bindings = nil
}

// Eval parses and runs one cell. interactive controls auto-display only; the
// driver always returns the value so an embedder can choose.
func (r *Repl) Eval(src string, interactive bool) (EvalResult, error) {
	src = translateShellLine(src)
	cell, msg := parseReplCell(src)
	if cell == nil {
		if incomplete, reason := syntax.Incomplete([]byte(src)); incomplete {
			return EvalResult{Incomplete: true, Message: reason}, nil
		}
		r.setLastStatus(1)
		return EvalResult{Message: msg, Status: 1}, nil
	}
	if cell.decls == "" && len(cell.body) == 0 && !cell.lastExpr {
		return EvalResult{}, nil
	}
	for _, let := range cell.lets {
		if strings.HasPrefix(let.name, "__repl_") {
			return EvalResult{Message: fmt.Sprintf("repl: `%s` is reserved for the evaluator", let.name), Status: 1}, nil
		}
		if r.binding(let.name) >= 0 {
			return EvalResult{Message: fmt.Sprintf("repl: `%s` is already a session binding; choose a fresh name", let.name), Status: 1}, nil
		}
	}
	needShell := replNeedsShell(src)
	declsTry := r.decls
	if needShell && !r.shell {
		if declsTry != "" {
			declsTry += "\n"
		}
		declsTry += replShellPrelude
	}
	if cell.decls != "" {
		if declsTry != "" {
			declsTry += "\n"
		}
		declsTry += cell.decls
	}
	if len(cell.body) == 0 && !cell.lastExpr {
		// Declarations persist without running a cell.
		if err := r.validate(declsTry); err != nil {
			r.setLastStatus(1)
			return EvalResult{Message: err.Error(), Status: 1}, nil
		}
		r.decls = declsTry
		if needShell {
			r.shell = true
		}
		return EvalResult{}, nil
	}

	cellName := fmt.Sprintf("__repl_cell_%d", r.seq)
	first := replWrapper(declsTry, cellName, r.bindings, "", nil, cell.body, cell.last, false, "__repl_probe")
	m, res, tree, _, err := r.check(first)
	if err != nil {
		return EvalResult{}, err
	}
	if res.HasErrors() {
		r.setLastStatus(1)
		return EvalResult{Message: r.renderDiags(res, tree, cell), Status: 1}, nil
	}
	letEnts, _, _ := generatedCellLocals(m, res, tree, cellName)
	lastType := sem.TypeID(0)
	if ent := letEnts["__repl_probe"]; ent != 0 {
		lastType = res.Entity(ent).Type
	}
	var lastTyp string
	lastPayload := lastType
	lastIsResult := false
	lastNever := lastType == sem.TyNever
	if lastType != 0 && lastType != sem.TyUnit && lastType != sem.TyNever {
		if payload, _, ok := res.Types.IsResult(lastType); ok {
			lastIsResult = true
			lastPayload = payload
		}
		if lastPayload != sem.TyUnit && lastPayload != sem.TyNever {
			lastTyp = replTypeString(res, lastPayload)
		}
	}
	planRun := lastTyp == "shell.Plan"
	if planRun {
		lastTyp = ""
	}

	out := make([]replOutField, 0, len(r.bindings)+len(cell.lets)+1)
	specs := make([]replOutSpec, 0, len(r.bindings)+len(cell.lets)+1)
	for i := range r.bindings {
		out = append(out, replOutField{name: r.bindings[i].Name, typ: r.bindings[i].Type})
		specs = append(specs, replOutSpec{name: r.bindings[i].Name, kind: replOutBinding})
	}
	for _, let := range cell.lets {
		if strings.HasPrefix(let.name, "__repl_") {
			continue
		}
		ent := letEnts[let.name]
		if ent == 0 {
			continue
		}
		t := res.Entity(ent).Type
		out = append(out, replOutField{name: let.name, typ: replTypeString(res, t)})
		specs = append(specs, replOutSpec{name: let.name, kind: replOutNew, mut: let.mut, typ: replTypeString(res, t)})
	}
	if lastTyp != "" {
		out = append(out, replOutField{name: "__repl_last", typ: lastTyp})
		specs = append(specs, replOutSpec{name: "__repl_last", kind: replOutLast})
	}
	if planRun {
		out = append(out, replOutField{name: "__repl_status", typ: "Int"})
		specs = append(specs, replOutSpec{name: "__repl_status", kind: replOutPlanStatus})
	}

	var finalProg *mir.Program
	var finalRes *sem.Result
	var finalTree *syntax.Tree
	var finalMod *Module
	for {
		outName := ""
		if len(out) > 0 {
			outName = fmt.Sprintf("__repl_out_%d", r.seq)
		}
		lastVar := ""
		lastExpr := cell.last
		lastResult := lastIsResult
		if cell.last != "" {
			lastVar = "_"
			if lastTyp != "" {
				lastVar = "__repl_last"
			}
			if lastNever {
				lastVar = "!"
			}
		}
		if planRun {
			lastVar = "__repl_status"
			if lastIsResult {
				lastExpr = "__repl_run_plan((" + cell.last + ")?)"
			} else {
				lastExpr = "__repl_run_plan(" + cell.last + ")"
			}
			lastResult = true
		}
		finalSrc := replWrapper(declsTry, cellName, r.bindings, outName, out, cell.body, lastExpr, lastResult, lastVar)
		fm, fres, ftree, fprog, err := r.finalize(finalSrc, cellName)
		if err != nil {
			return EvalResult{}, err
		}
		if !fres.HasErrors() {
			finalMod, finalRes, finalTree, finalProg = fm, fres, ftree, fprog
			break
		}
		drop := replOutputErrors(fres, ftree, outName, specs)
		if len(drop) == 0 {
			r.setLastStatus(1)
			return EvalResult{Message: r.renderDiags(fres, ftree, cell), Status: 1}, nil
		}
		var nextOut []replOutField
		var nextSpecs []replOutSpec
		for i, spec := range specs {
			if drop[i] {
				continue
			}
			nextOut = append(nextOut, out[i])
			nextSpecs = append(nextSpecs, spec)
		}
		out, specs = nextOut, nextSpecs
	}
	_ = finalMod
	_ = finalRes
	_ = finalTree
	if finalProg == nil || finalProg.Entry == nil {
		return EvalResult{}, fmt.Errorf("repl: cell %q did not lower", cellName)
	}
	if r.sess == nil {
		s, err := vm.NewSession(finalProg, r.vmOpts, r.out, r.err)
		if err != nil {
			return EvalResult{}, err
		}
		r.sess = s
	} else if err := r.sess.UseProgram(finalProg); err != nil {
		r.setLastStatus(1)
		return EvalResult{Message: "repl: session state is incompatible with this cell: " + err.Error(), Status: 1}, nil
	} else {
		for i := range r.bindings {
			r.sess.Remap(r.bindings[i].Value)
		}
	}
	args := make([]vm.Value, len(r.bindings))
	for i := range r.bindings {
		args[i] = r.bindings[i].Value.Retain()
	}
	resVal, runErr := r.sess.RunCell(finalProg.Entry, args)
	r.sess.Quiesce()
	if runErr != nil {
		return r.failedCell(runErr), nil
	}
	ok, isResult := resVal.ResultOk()
	if isResult && !ok {
		msg := resVal.ErrorMessage()
		resVal.Release()
		r.setLastStatus(1)
		if msg == "" {
			msg = "cell failed"
		}
		return EvalResult{Message: msg, Status: 1}, nil
	}
	payload := resVal
	if isResult {
		payload, _ = resVal.Field(0)
		resVal.Release()
	}
	fields := payload.TakeFields()
	if len(fields) != len(specs) {
		for i := range fields {
			fields[i].Release()
		}
		r.setLastStatus(1)
		return EvalResult{}, fmt.Errorf("repl: cell returned %d fields, expected %d", len(fields), len(specs))
	}
	display := ""
	cellStatus := 0
	for i, spec := range specs {
		v := fields[i]
		switch spec.kind {
		case replOutBinding:
			if j := r.binding(spec.name); j >= 0 {
				r.bindings[j].Value.Release()
				r.bindings[j].Value = v
			}
		case replOutNew:
			r.bindings = append(r.bindings, ReplBinding{Name: spec.name, Type: spec.typ, Mut: spec.mut, Value: v})
		case replOutLast:
			display = v.Display()
			v.Release()
		case replOutPlanStatus:
			if n, ok := v.IntValue(); ok {
				cellStatus = int(n)
			}
			v.Release()
		}
	}
	// A binding the cell moved is not in the return record. Drop the host's
	// retained reference now; the cell already owns what escaped.
	kept := make(map[string]bool, len(specs))
	for _, spec := range specs {
		kept[spec.name] = true
	}
	next := r.bindings[:0]
	for _, b := range r.bindings {
		if kept[b.Name] {
			next = append(next, b)
			continue
		}
		b.Value.Release()
	}
	r.bindings = next
	r.decls = declsTry
	if needShell {
		r.shell = true
	}
	r.seq++
	r.setLastStatus(cellStatus)
	if !interactive {
		display = ""
	}
	return EvalResult{Display: display, Status: 0}, nil
}

type replOutKind int

const (
	replOutBinding replOutKind = iota
	replOutNew
	replOutLast
	replOutPlanStatus
)

type replOutSpec struct {
	name string
	typ  string
	mut  bool
	kind replOutKind
}

func (r *Repl) binding(name string) int {
	for i := range r.bindings {
		if r.bindings[i].Name == name {
			return i
		}
	}
	return -1
}

func (r *Repl) setLastStatus(code int) {
	i := r.binding("lastStatus")
	if i < 0 {
		if r.sess != nil {
			r.bindings = append(r.bindings, ReplBinding{Name: "lastStatus", Type: "Int", Mut: true, Value: r.sess.Int(int64(code))})
		}
		return
	}
	r.bindings[i].Value.Release()
	r.bindings[i].Value = r.sess.Int(int64(code))
}

func (r *Repl) failedCell(err error) EvalResult {
	if x, ok := err.(*vm.SessionExit); ok {
		return EvalResult{Exit: true, ExitCode: x.Code, Status: x.Code}
	}
	r.setLastStatus(1)
	if _, ok := err.(*vm.Cancelled); ok {
		return EvalResult{Message: "cancelled", Status: 1}
	}
	return EvalResult{Message: err.Error(), Status: 1}
}

func (r *Repl) check(src string) (*Module, *sem.Result, *syntax.Tree, *Package, error) {
	m, err := r.loadModule(src)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	res, err := CheckLazyRoot(m)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	pkg, tree := generatedFile(m, r.entry)
	if tree == nil {
		return nil, nil, nil, nil, fmt.Errorf("repl: generated entry %q was not indexed", r.entry)
	}
	return m, res, tree, pkg, nil
}

func (r *Repl) validate(decls string) error {
	src := decls + "\n"
	m, err := r.loadModule(src)
	if err != nil {
		return err
	}
	res, err := CheckLazyRoot(m)
	if err != nil {
		return err
	}
	if res.HasErrors() {
		return fmt.Errorf("%s", strings.TrimSpace(res.Render()))
	}
	return nil
}

func (r *Repl) finalize(src, cellName string) (*Module, *sem.Result, *syntax.Tree, *mir.Program, error) {
	m, err := r.loadModule(src)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	pkg, tree := generatedFile(m, r.entry)
	if pkg == nil || tree == nil {
		return nil, nil, nil, nil, fmt.Errorf("repl: generated package is missing")
	}
	res, err := CheckLazyRoot(m)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	ent := res.PackageMember(res.PackageByPath(pkg.Path), cellName)
	if ent == 0 {
		return nil, nil, nil, nil, fmt.Errorf("repl: generated cell %q is missing from the final program", cellName)
	}
	a, err := m.LowerCheckedRoot(res, ent)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return m, a.Res, tree, a.Prog, nil
}

func (r *Repl) loadModule(src string) (*Module, error) {
	opts := r.load
	opts.Entry = r.entry
	opts.Compiler.Cwd = r.root
	opts.Overlay = map[string][]byte{filepath.Join(r.root, r.entry): []byte(src)}
	return LoadModule(r.root, opts)
}

func (r *Repl) renderDiags(res *sem.Result, tree *syntax.Tree, cell *replCell) string {
	if tree == nil || cell == nil {
		return res.Render()
	}
	// Diagnostics in the generated wrapper name internal lines. Map a copied
	// body line back to the cell; wrapper-only lines are not user source.
	type cellLine struct {
		n    int
		text string
	}
	var cellLines []cellLine
	for n, line := range strings.Split(string(cell.file.Src), "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			cellLines = append(cellLines, cellLine{n: n + 1, text: t})
		}
	}
	findCellLine := func(text string) (int, bool) {
		var best int
		var bestText string
		for _, c := range cellLines {
			if c.text == text || strings.Contains(text, c.text) && len(c.text) > len(bestText) {
				best, bestText = c.n, c.text
			}
		}
		return best, bestText != ""
	}
	var b strings.Builder
	for _, fi := range res.Files {
		if fi.Tree == nil {
			continue
		}
		if fi.Tree != tree {
			b.WriteString(diag.RenderAll(res, fi.Diags))
			continue
		}
		var mapped []diag.Diagnostic
		for _, d := range fi.Diags {
			line := tree.File.Line(d.Loc.Span.Start)
			text := strings.TrimSpace(tree.File.LineText(line))
			cellLine, ok := findCellLine(text)
			if !ok {
				continue
			}
			d.Loc = diag.At(cell.file, cellLineSpan(cell.file, cellLine))
			d.Notes = nil
			mapped = append(mapped, d)
		}
		b.WriteString(diag.RenderAll(cell.file, mapped))
	}
	return b.String()
}

func cellLineSpan(file *token.File, line int) token.Span {
	return file.LineSpan(line)
}

// replTypeString renders a type for the generated entry file. The checker's
// TypeString omits package qualification; the prelude imports std/shell as
// `shell`, so the REPL's shell values need that qualifier.
func replTypeString(res *sem.Result, t sem.TypeID) string {
	s := res.TypeString(t)
	switch s {
	case "Plan", "Shell", "PipelineResult", "Item", "Stage", "Captured":
		return "shell." + s
	}
	return s
}

// translateShellLine accepts the small set of shell-shaped commands the REPL
// exposes. Everything else is kigumi source; this never interprets POSIX
// syntax generally.
func translateShellLine(src string) string {
	trimmed := strings.TrimSpace(src)
	if trimmed == "" || strings.Contains(trimmed, "\n") {
		return src
	}
	fields := strings.Fields(trimmed)
	switch fields[0] {
	case "pwd":
		if len(fields) == 1 {
			return "print pwd()"
		}
	case "cd":
		if len(fields) >= 2 {
			return "cd(" + quoteKigumi(shellArg(strings.TrimSpace(strings.TrimPrefix(trimmed, "cd")))) + ")"
		}
	case "export":
		if len(fields) >= 2 {
			name, value, ok := strings.Cut(fields[1], "=")
			if ok {
				return "setEnv(" + quoteKigumi(name) + ", " + quoteKigumi(value) + ")"
			}
		}
	case "unset":
		if len(fields) == 2 {
			return "unsetEnv(" + quoteKigumi(fields[1]) + ")"
		}
	case "exit":
		if len(fields) == 1 {
			return "exit(0)"
		}
		if len(fields) == 2 {
			return "exit(" + fields[1] + ")"
		}
	}
	return src
}

func shellArg(s string) string {
	if len(s) >= 2 && (s[0] == '\'' && s[len(s)-1] == '\'' || s[0] == '"' && s[len(s)-1] == '"') {
		return s[1 : len(s)-1]
	}
	return s
}

func quoteKigumi(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(s) + `"`
}

// Completions returns session binding and declaration names, the REPL
// prelude, and matching paths. The editor gets whole replacements.
func (r *Repl) Completions(prefix string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if strings.HasPrefix(s, prefix) && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, b := range r.bindings {
		add(b.Name)
	}
	for _, n := range []string{"cd", "pwd", "setEnv", "unsetEnv", "env", "sh", "exit", "print", "eprint", "host"} {
		add(n)
	}
	for _, n := range replDeclNames(r.decls) {
		add(n)
	}
	if prefix != "" && (strings.Contains(prefix, "/") || strings.HasPrefix(prefix, ".")) {
		if matches, _ := filepath.Glob(prefix + "*"); len(matches) > 0 {
			for _, m := range matches {
				if strings.HasSuffix(m, "/") {
					add(m)
				} else {
					add(m)
				}
			}
		}
	}
	return out
}

func replDeclNames(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "fn", "type", "const", "interface":
			name := strings.TrimSuffix(f[1], "(")
			if i := strings.IndexAny(name, "([<"); i >= 0 {
				name = name[:i]
			}
			if name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

func replNeedsShell(src string) bool {
	return strings.Contains(src, `$"`) || strings.Contains(src, "shell") || strings.Contains(src, "Plan") || strings.Contains(src, "sh(")
}

// Cancel interrupts the running cell, if any.
func (r *Repl) Cancel() {
	if r != nil && r.sess != nil {
		r.sess.Cancel()
	}
}

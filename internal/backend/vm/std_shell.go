package vm

import (
	"io"
	"strings"

	"kigumi/internal/backend/vm/host/shell"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// stdFn finds a std function by package path and name.
func (m *Machine) stdFn(pkg, name string) sem.EntityID {
	return m.r.PackageMember(m.r.PackageByPath(pkg), name)
}

// callStd runs a std function with a Kigumi body on lent arguments.
func (m *Machine) callStd(pkg, name string, args []*obj) *obj {
	fn := m.stdFn(pkg, name)
	if f, ok := m.lowered(fn); ok {
		return m.call(f, args)
	}
	return m.callPrim(fn, m.stdKey(fn), nil, args)
}

// shellPlan lowers a shell literal the way llgen does: std/shell.planNew
// and planPush build the Plan from Item variants.
func (m *Machine) shellPlan(in *midir.Inst, args []*obj) *obj {
	item := m.r.PackageMember(m.r.PackageByPath("std/shell"), "Item")
	variant := func(name string, payload []*obj) *obj {
		return mkVariant(m.variantOf(item, name), payload)
	}
	plan := m.callStd("std/shell", "planNew", nil)
	ai := 0
	for _, s := range in.Strs {
		var it *obj
		switch {
		case s == "|":
			it = variant("Pipe", nil)
		case s == " ":
			it = variant("Word", nil)
		case s == "$":
			it = variant("Text", []*obj{mkStr([]byte(m.display(args[ai])))})
			ai++
		case s[0] == 'r':
			op, fd, _ := strings.Cut(s[1:], ":")
			it = variant("Redirect", []*obj{mkInt(atoi(op), 64|256), mkInt(atoi(fd), 64|256)})
		default:
			it = variant("Text", []*obj{mkStr([]byte(s[1:]))})
		}
		plan = m.callStd("std/shell", "planPush", []*obj{plan, it})
	}
	return plan
}

func atoi(s string) int64 {
	var n int64
	for _, c := range s {
		n = n*10 + int64(c-'0')
	}
	return n
}

// planOf turns a Plan record into the host plan by asking std/shell.stages
// for the stage records, so word joining lives in one place. Every filter is
// validated by the same std/shell.filterProblem the native runner uses, so
// both engines reject the same options with the same message.
func (m *Machine) planOf(v *obj, cwd string) (*shell.Plan, string) {
	stages := m.callStd("std/shell", "stages", []*obj{v})
	defer m.release(stages)
	p := &shell.Plan{}
	for _, sv := range deref(stages).fields {
		rec := deref(sv)
		var st shell.Stage
		for _, a := range deref(rec.fields[0]).fields {
			st.Argv = append(st.Argv, string(deref(a).str()))
		}
		for _, r := range deref(rec.fields[1]).fields {
			rr := deref(r)
			st.Redirs = append(st.Redirs, shell.Redirect{Op: uint32(deref(rr.fields[0]).i), Fd: int(deref(rr.fields[1]).i), Target: string(deref(rr.fields[2]).str())})
		}
		if opt := deref(rec.fields[2]); len(opt.fields) == 1 {
			f := deref(opt.fields[0])
			problem := m.callStd("std/shell", "filterProblem", []*obj{f})
			text := string(deref(problem).str())
			m.release(problem)
			if text != "" {
				return nil, text
			}
			filter, why := m.filterOf(f, cwd)
			if why != "" {
				return nil, why
			}
			st.Filter = filter
		}
		p.Stages = append(p.Stages, st)
	}
	return p, ""
}

// filterOf turns a std/shell.Filter variant into the Go filter that runs it.
// The payloads are the primitives the ADT declares, in declaration order.
func (m *Machine) filterOf(f *obj, cwd string) (func(io.Reader, io.Writer) error, string) {
	name := m.r.Entity(f.ent).Name
	field := func(i int) *obj { return deref(f.fields[i]) }
	boolAt := func(i int) bool { return field(i).b }
	intAt := func(i int) int64 { return field(i).i }
	strAt := func(i int) string { return string(field(i).str()) }
	switch name {
	case "Cat":
		return shell.CatFilter(), ""
	case "Head":
		return shell.HeadFilter(int(intAt(0))), ""
	case "Tail":
		return shell.TailFilter(int(intAt(0))), ""
	case "Wc":
		return shell.WcFilter(boolAt(0), boolAt(1), boolAt(2), boolAt(3)), ""
	case "Grep":
		return shell.GrepFilter(strAt(0)), ""
	case "Sort":
		return shell.SortFilter(boolAt(0), boolAt(1)), ""
	case "Uniq":
		return shell.UniqFilter(boolAt(0), boolAt(1), boolAt(2)), ""
	case "Cut":
		fields := []int64{}
		for _, v := range field(1).fields {
			fields = append(fields, deref(v).i)
		}
		return shell.CutFilter([]byte(strAt(0)), fields), ""
	case "Tr":
		return shell.TrFilter([]byte(strAt(0)), []byte(strAt(1)), boolAt(2), boolAt(3)), ""
	case "Tee":
		return shell.TeeFilter(strAt(0), boolAt(1), cwd), ""
	}
	return nil, "unknown filter stage"
}

func (m *Machine) registerShell() {
	h := m.host
	h["os.Host.shell"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Shell", nil) }
	h["shell.capture"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		plan, problem := m.planOf(a[0], m.machineCwd())
		if problem != "" {
			return m.errMsg(problem)
		}
		out, errOut, _, err := shell.Run(plan, m.machineCwd())
		if err != nil {
			return m.errMsg(err.Error())
		}
		captured := m.r.Types.Node(m.resultPayload(fn)).Ent
		return m.ok(mkRecord(captured, []*obj{mkBytes(out), mkBytes(errOut)}))
	}
	h["shell.run"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		plan, problem := m.planOf(a[0], m.machineCwd())
		if problem != "" {
			return m.errMsg(problem)
		}
		out, errOut, statuses, err := shell.Run(plan, m.machineCwd())
		if err != nil {
			return m.errMsg(err.Error())
		}
		m.writeOut(out)
		m.writeErr(errOut)
		codes := mkArray(nil)
		for _, st := range statuses {
			codes.fields = append(codes.fields, mkInt(int64(st), 64|256))
		}
		result := m.r.Types.Node(m.resultPayload(fn)).Ent
		return m.ok(mkRecord(result, []*obj{codes}))
	}
	h["shell.runHere"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		plan, problem := m.planOf(a[0], m.machineCwd())
		if problem != "" {
			return m.errMsg(problem)
		}
		statuses, err := shell.RunHere(plan, m.machineCwd(), m.opts.Stdin, m.out, m.err)
		if err != nil {
			return m.errMsg(err.Error())
		}
		codes := mkArray(nil)
		for _, st := range statuses {
			codes.fields = append(codes.fields, mkInt(int64(st), 64|256))
		}
		result := m.r.Types.Node(m.resultPayload(fn)).Ent
		return m.ok(mkRecord(result, []*obj{codes}))
	}
}

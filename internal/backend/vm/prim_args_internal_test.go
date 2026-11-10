package vm

import (
	"slices"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// TestPrimArgTableCoversContainerPrimitives enumerates the primitives the VM answers in Go (bodiless std functions and
// host functions) that touch a container: every integer argument needs a role, and every callback one of the two callback roles.
func TestPrimArgTableCoversContainerPrimitives(t *testing.T) {
	r := sem.CheckWith(&sem.Module{Packages: semfixture.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	m := &Machine{sh: newShared()}
	m.registerAll()
	ints, fns := map[string]map[int]bool{}, map[string][]int{}
	checked := 0
	for id := range r.Entities {
		ent := sem.EntityID(id)
		if e := r.Entity(ent); e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 {
			continue
		}
		info := r.Fn(ent)
		key := opdef.StdKey(r, ent, true)
		if _, hosted := m.host[key]; info.Abi != "" || info.Body != 0 && !hosted {
			continue
		}
		params := info.Params
		if info.Recv != sem.RecvNone {
			params = append([]sem.EntityID{info.SelfParam}, params...)
		}
		ints[key] = map[int]bool{}
		touches := !scalarType(r, r.Types.Node(info.Sig).Elem)
		var callbacks []int
		for i, p := range params {
			pt := r.Entity(p).Type
			if r.Types.IsInteger(pt) {
				ints[key][i] = true
			}
			if r.FnShapeOf(pt) != 0 {
				callbacks = append(callbacks, i)
				fns[key] = append(fns[key], i)
			}
			touches = touches || !scalarType(r, pt)
		}
		if hostEffect(key) || !touches || info.Recv != sem.RecvNone && scalarType(r, r.Entity(info.SelfParam).Type) {
			continue
		}
		for _, at := range callbacks {
			if !slices.ContainsFunc(primArgTable[key], func(a primArg) bool {
				return a.at == at && (a.role == roleBranches || a.role == roleYields)
			}) {
				t.Errorf("%s: callback argument %d is neither roleBranches nor roleYields in primArgTable", key, at)
			}
		}
		roles := map[int]bool{}
		for _, a := range primArgTable[key] {
			roles[a.at] = true
		}
		for at := range ints[key] {
			checked++
			if !roles[at] {
				t.Errorf("%s: integer argument %d has no role in primArgTable", key, at)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no container primitive with an integer argument found")
	}
	for key, args := range primArgTable {
		for _, a := range args {
			if !ints[key][a.at] && !slices.Contains(fns[key], a.at) {
				t.Errorf("primArgTable: %s argument %d is not an integer or callback argument of a primitive", key, a.at)
			}
		}
	}
}

func scalarType(r *sem.Result, t sem.TypeID) bool {
	return r.Types.IsNumeric(t) || t == sem.TyBool || t == sem.TyChar || t == sem.TyUnit || t == sem.TyNever
}

package mir

import (
	"slices"
	"strconv"
	"strings"
	"sync"

	"kigumi/internal/sem"
)

type unwindMemo struct {
	mu    sync.Mutex
	plans map[*Func]*UnwindPlan
}

// UnwindPlan lists, per instruction, the owned locals a panic there must release: those used or dropped after it, bar its own result.
type UnwindPlan struct {
	// Sets[0] is empty; each set runs from the last declared local to the first.
	Sets [][]LocalID
	// At is -1 where the instruction cannot panic.
	At [][]int32
}

func (u *UnwindPlan) SetAt(b BlockID, pc int) []LocalID {
	if int(b) >= len(u.At) || pc >= len(u.At[b]) || u.At[b][pc] < 0 {
		return nil
	}
	return u.Sets[u.At[b][pc]]
}

func (p *Program) UnwindPlanOf(f *Func) *UnwindPlan {
	p.unwind.mu.Lock()
	defer p.unwind.mu.Unlock()
	if plan, ok := p.unwind.plans[f]; ok {
		return plan
	}
	if p.unwind.plans == nil {
		p.unwind.plans = map[*Func]*UnwindPlan{}
	}
	plan := planUnwind(f, p.R)
	p.unwind.plans[f] = plan
	return plan
}

func cannotPanic(op Opcode) bool {
	switch op {
	case OpConst, OpUnit, OpAlias, OpMove, OpCopy, OpNop, OpFnItem, OpShare:
		return true
	}
	return false
}

type bitset []uint64

func (b bitset) has(l LocalID) bool { return b[l/64]&(1<<(l%64)) != 0 }
func (b bitset) set(l LocalID)      { b[l/64] |= 1 << (l % 64) }
func (b bitset) clear(l LocalID)    { b[l/64] &^= 1 << (l % 64) }

func planUnwind(f *Func, r *sem.Result) *UnwindPlan {
	plan := &UnwindPlan{Sets: [][]LocalID{nil}, At: make([][]int32, len(f.Blocks))}
	for i := range f.Blocks {
		plan.At[i] = make([]int32, len(f.Blocks[i].Insts))
		for j := range plan.At[i] {
			plan.At[i][j] = -1
		}
	}
	if f.Async {
		return plan
	}
	owned := owningLocals(f, r)
	words := (len(f.Locals) + 63) / 64
	liveIn := make([]bitset, len(f.Blocks))
	for i := range liveIn {
		liveIn[i] = make(bitset, words)
	}
	walk := func(bid int, visit func(i int, live bitset)) bitset {
		blk := &f.Blocks[bid]
		live := make(bitset, words)
		for _, t := range blk.Term.Targets {
			for w := range live {
				live[w] |= liveIn[t][w]
			}
		}
		for _, a := range blk.Term.Args {
			if owned[a] {
				live.set(a)
			}
		}
		for i := len(blk.Insts) - 1; i >= 0; i-- {
			in := &blk.Insts[i]
			if visit != nil {
				visit(i, live)
			}
			if shapes[in.Op].result {
				live.clear(in.Dst)
			}
			for _, a := range in.Args {
				if owned[a] {
					live.set(a)
				}
			}
		}
		return live
	}
	for changed := true; changed; {
		changed = false
		for bid := len(f.Blocks) - 1; bid >= 0; bid-- {
			if live := walk(bid, nil); !slices.Equal(live, liveIn[bid]) {
				liveIn[bid] = live
				changed = true
			}
		}
	}
	parent := aliasParents(f)
	interned := map[string]int32{}
	for bid := range f.Blocks {
		walk(bid, func(i int, live bitset) {
			in := &f.Blocks[bid].Insts[i]
			if cannotPanic(in.Op) {
				return
			}
			plan.At[bid][i] = 0
			var set []LocalID
			for l := len(f.Locals) - 1; l >= 0; l-- {
				id := LocalID(l)
				if !live.has(id) || shapes[in.Op].result && in.Dst == id {
					continue
				}
				if sharesOwner(id, parent, owned, live, in) {
					continue
				}
				set = append(set, id)
			}
			if len(set) == 0 {
				return
			}
			key := keyOf(set)
			idx, ok := interned[key]
			if !ok {
				idx = int32(len(plan.Sets))
				plan.Sets = append(plan.Sets, set)
				interned[key] = idx
			}
			plan.At[bid][i] = idx
		})
	}
	return plan
}

func keyOf(set []LocalID) string {
	var sb strings.Builder
	for _, l := range set {
		sb.WriteString(strconv.Itoa(int(l)))
		sb.WriteByte(',')
	}
	return sb.String()
}

// An alias holds the reference of the local it copies: an unowned source means nothing to release, a live one releases it once.
func sharesOwner(id LocalID, parent map[LocalID]LocalID, owned map[LocalID]bool, live bitset, in *Inst) bool {
	src := id
	for range len(parent) + 1 {
		next, ok := parent[src]
		if !ok {
			return false
		}
		src = next
		if !owned[src] || live.has(src) && !(shapes[in.Op].result && in.Dst == src) {
			return true
		}
	}
	return false
}

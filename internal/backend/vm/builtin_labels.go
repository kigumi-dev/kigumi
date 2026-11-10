package vm

// builtinLabel is how a builtin's arguments reach its sinks and its result under SecretSan.
type builtinLabel struct {
	args   []primArg
	result resultRule
	// out marks arguments that leave the program (output, a panic message).
	out bool
}

var atomicLabel = builtinLabel{args: []primArg{valueArg(0), valueArg(1)}, result: resultSlot}

// builtinTable gives every builtin the VM runs its label rule.
var builtinTable = map[string]builtinLabel{
	"print":                  {out: true, result: resultNone},
	"eprint":                 {out: true, result: resultNone},
	"panic":                  {out: true, result: resultNone},
	"host":                   {result: resultNone},
	"task.close":             {result: resultNone},
	"alloc.push":             {result: resultNone},
	"fixed.fill":             {args: []primArg{valueArg(0), sizeArg(1)}, result: resultShapes},
	"iter.len":               {result: resultLen},
	"iter.at":                {args: []primArg{atIndex(1, 0)}, result: resultShapes},
	"iter.at_ref":            {args: []primArg{atIndex(1, 0)}, result: resultShapes},
	"iter.at_move":           {args: []primArg{atIndex(1, 0)}, result: resultShapes},
	"direct.eq":              {result: resultDeep},
	"direct.hash":            {result: resultDeep},
	"cmp.unordered":          {result: resultAll},
	"cmp.guard":              {result: resultAll},
	"atomic.load":            atomicLabel,
	"atomic.store":           atomicLabel,
	"atomic.exchange":        atomicLabel,
	"atomic.compareExchange": atomicLabel,
	"atomic.add":             atomicLabel,
	"atomic.sub":             atomicLabel,
	"atomic.and":             atomicLabel,
	"atomic.or":              atomicLabel,
	"atomic.xor":             atomicLabel,
	"volatile.read":          {args: []primArg{addressArg(0)}, result: resultSlot},
	"volatile.write":         {args: []primArg{addressArg(0), valueArg(1)}, result: resultNone},
}

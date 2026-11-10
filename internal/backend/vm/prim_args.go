package vm

// argRole is what an integer argument of a container primitive does.
type argRole uint8

const (
	// roleIndex selects a position in argument `of`: an index sink and a trace event.
	roleIndex argRole = iota + 1
	roleSize
	// roleValue is data the primitive stores or combines, not a position.
	roleValue
	// roleAddress is a raw memory address: an address sink and a trace event.
	roleAddress
	// roleBranches is a callback whose result the primitive branches on (a predicate, a comparator): a branch sink.
	roleBranches
	// roleYields is a callback whose result the primitive hands back or stores, carrying its own label.
	roleYields
)

type primArg struct {
	at   int
	role argRole
	of   int
}

func atIndex(at, of int) primArg { return primArg{at: at, role: roleIndex, of: of} }
func sizeArg(at int) primArg     { return primArg{at: at, role: roleSize} }
func valueArg(at int) primArg    { return primArg{at: at, role: roleValue} }
func addressArg(at int) primArg  { return primArg{at: at, role: roleAddress, of: -1} }
func branchArg(at int) primArg   { return primArg{at: at, role: roleBranches} }
func yieldArg(at int) primArg    { return primArg{at: at, role: roleYields} }

// primArgTable gives each integer argument of a primitive touching a container its role, by argument position.
var primArgTable = map[string][]primArg{
	"array.Array.get":                   {atIndex(1, 0)},
	"array.Array.with":                  {atIndex(1, 0), yieldArg(2)},
	"array.Array.withMut":               {atIndex(1, 0), yieldArg(2)},
	"array.Array.tryMap":                {yieldArg(1)},
	"array.Array.tryFilter":             {branchArg(1)},
	"array.trySortedBy":                 {branchArg(1)},
	"alloc.Box.with":                    {yieldArg(1)},
	"alloc.Box.withMut":                 {yieldArg(1)},
	"alloc.Shared.with":                 {yieldArg(1)},
	"alloc.Shared.withMut":              {yieldArg(1)},
	"array.Array.takeAt":                {atIndex(1, 0)},
	"array.Array.tryInsertAt":           {atIndex(1, 0)},
	"array.Array.trySlice":              {atIndex(1, 0), atIndex(2, 0)},
	"array.Array.tryCopyRange":          {atIndex(1, 0), atIndex(2, 0)},
	"array.Array.growBy":                {sizeArg(1)},
	"array.Array.tryWithCapacity":       {sizeArg(0)},
	"prelude.FixedArray.withMut":        {atIndex(1, 0), yieldArg(2)},
	"prelude.Bytes.get":                 {atIndex(1, 0)},
	"prelude.Bytes.trySlice":            {atIndex(1, 0), atIndex(2, 0)},
	"prelude.Bytes.tryZeros":            {sizeArg(0)},
	"prelude.ByteBuffer.tryNew":         {sizeArg(0)},
	"prelude.ByteBuffer.set":            {atIndex(1, 0), valueArg(2)},
	"prelude.ByteBuffer.copyFrom":       {atIndex(1, 0), atIndex(3, 2), atIndex(4, 2)},
	"prelude.ByteBuffer.copyFromString": {atIndex(1, 0), atIndex(3, 2), atIndex(4, 2)},
	"prelude.String.trySliceBytes":      {atIndex(1, 0), atIndex(2, 0)},
	"entropy.Entropy.bytes":             {sizeArg(1)},
	"task.threadPoolExecutor":           {sizeArg(0)},
	"prelude.Char.fromInt":              {valueArg(0)},
	"mem.volatileReadAt":                {addressArg(0)},
	"mem.volatileWriteAt":               {addressArg(0)},
}

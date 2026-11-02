package sem

// PartStep leads from a value to the parts that can hold what a coercion erases: field Index of a record when Field
// is set, else every part a named type holds its type argument Index in.
type PartStep struct {
	Index int
	Field bool
}

type partPath []PartStep

func (p partPath) arg(i int) partPath { return append(p[:len(p):len(p)], PartStep{Index: i}) }

func (p partPath) then(q partPath) partPath { return append(p[:len(p):len(p)], q...) }

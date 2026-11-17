package ir

type DropKind uint8

const (
	DropWhole DropKind = iota
	DropPartial
	DropConditional
)

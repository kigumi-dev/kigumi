package borrow

import (
	"kigumi/internal/compiler/sem"
)

type TypeFacts struct {
	r    *sem.Result
	memo borrowTypeMemo
}

func NewTypeFacts(r *sem.Result) *TypeFacts { return &TypeFacts{r: r} }

func (c *borrowCheck) facts() *TypeFacts {
	if c.types == nil {
		c.types = NewTypeFacts(c.p.R)
	}
	return c.types
}

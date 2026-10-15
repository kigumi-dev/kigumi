package mir

import (
	"fmt"
	"sync/atomic"
)

// Transform is one pass over a program: a fold, a rewrite, an
// optimization.
type Transform struct {
	Name string
	Run  func(*Program) error
}

var verifyEachPass atomic.Bool

// SetVerifyEachPass turns per-pass verification on for every program, as if each had VerifyEach set.
func SetVerifyEachPass(on bool) { verifyEachPass.Store(on) }

// Apply runs each transform and verifies the result; with VerifyEach it
// also verifies before the passes and after every one of them.
func (p *Program) Apply(passes ...Transform) error {
	each := p.VerifyEach || verifyEachPass.Load()
	// The builder's borrow checks fill this cache from MIR the passes then rewrite.
	p.invalidateUnowned()
	if each {
		if err := p.Verify(); err != nil {
			return fmt.Errorf("before passes: %w", err)
		}
	}
	for _, t := range passes {
		if err := t.Run(p); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
		if each {
			if err := p.Verify(); err != nil {
				return fmt.Errorf("after %s: %w", t.Name, err)
			}
		}
	}
	if !each {
		if err := p.Verify(); err != nil {
			return fmt.Errorf("after passes: %w", err)
		}
	}
	return nil
}

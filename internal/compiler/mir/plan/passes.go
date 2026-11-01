package plan

import (
	"fmt"
	"sync/atomic"

	"kigumi/internal/compiler/mir"
	mirverify "kigumi/internal/compiler/mir/verify"
)

// Transform is one pass over a program: a fold, a rewrite, an
// optimization.
type Transform struct {
	Name string
	Run  func(*mir.Program) error
}

var verifyEachPass atomic.Bool

// SetVerifyEachPass turns per-pass verification on for every program, as if each had VerifyEach set.
func SetVerifyEachPass(on bool) { verifyEachPass.Store(on) }

// Apply runs each transform and verifies the result; with VerifyEach it
// also verifies before the passes and after every one of them.
func Apply(p *mir.Program, passes ...Transform) error {
	var v mirverify.Session
	each := p.VerifyEach || verifyEachPass.Load()
	if each {
		if err := v.Run(p); err != nil {
			return fmt.Errorf("before passes: %w", err)
		}
	}
	for _, t := range passes {
		if err := t.Run(p); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
		if each {
			if err := v.Run(p); err != nil {
				return fmt.Errorf("after %s: %w", t.Name, err)
			}
		}
	}
	if !each {
		if err := v.Run(p); err != nil {
			return fmt.Errorf("after passes: %w", err)
		}
	}
	return nil
}

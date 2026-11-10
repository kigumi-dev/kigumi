package sem_test

import (
	"testing"
	"time"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

// TestClosureChainSendTime pins the Send judgement of a closure capturing a chain of closures to one walk of the chain:
// judged once per path it takes minutes at this depth, judged once per closure well under a second beyond std.
func TestClosureChainSendTime(t *testing.T) {
	fx := parseFixture(t, "../../../testdata/sem/pool_send_closure_chain.txtar")
	mod := buildModule(t, fx)
	done := make(chan *sem.Result, 1)
	start := time.Now()
	go func() { done <- sem.CheckWith(mod, schema.GenerateDerive) }()
	select {
	case res := <-done:
		if res.HasErrors() {
			t.Fatalf("the chain should check cleanly:\n%s", res.Render())
		}
		t.Logf("checked in %v", time.Since(start))
	case <-time.After(60 * time.Second):
		t.Fatalf("checking a closure chain of depth 40 took over a minute")
	}
}

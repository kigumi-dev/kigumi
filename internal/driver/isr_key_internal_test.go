package driver

import (
	"testing"

	"kigumi/internal/sem"
)

// A changed interrupt permission must re-key both the package cache and the analysis memo, so the handler is re-checked.
func TestInterruptConfigEntersKeys(t *testing.T) {
	m := &Module{target: HostTarget()}
	mod := &sem.Module{}
	env, analysis := m.cacheEnv(), analysisKey(m, mod)
	m.Manifest.Interrupt = sem.InterruptConfig{Root: []string{"mmio uart.ack"}}
	mod.Interrupt = m.Manifest.Interrupt
	if m.cacheEnv() == env || analysisKey(m, mod) == analysis {
		t.Fatal("the root interrupt configuration must enter the cache and analysis keys")
	}
	env, analysis = m.cacheEnv(), analysisKey(m, mod)
	m.Manifest.Interrupt.Handlers = map[string][]string{"main.tick": nil}
	mod.Interrupt = m.Manifest.Interrupt
	if m.cacheEnv() == env || analysisKey(m, mod) == analysis {
		t.Fatal("a handler's own record must enter the cache and analysis keys")
	}
}

func TestDependencyInterruptEntersKeys(t *testing.T) {
	m := &Module{target: HostTarget()}
	mod := &sem.Module{}
	env, analysis := m.cacheEnv(), analysisKey(m, mod)
	m.depInterrupt = []sem.DepInterrupt{{Module: "lib", Config: sem.InterruptConfig{Handlers: map[string][]string{"lib.tick": nil}}}}
	mod.DepInterrupt = m.depInterrupt
	if m.cacheEnv() == env || analysisKey(m, mod) == analysis {
		t.Fatal("a dependency's interrupt record must enter the cache and analysis keys")
	}
}

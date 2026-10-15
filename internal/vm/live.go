package vm

import "sync/atomic"

var (
	liveTrack                      atomic.Bool
	liveMade, liveFreed, liveStray atomic.Int64
	liveFreedUse                   atomic.Int64
)

// freedWord fills a recycled scalar's payload, so a write through a stale pointer is seen when the slot is reused.
const freedWord = 0x5a5a5a5a5a5a5a5a

// While live tracking is on a freed scalar is not recycled, so every stale pointer stays poisoned and a release through one is counted.
func noteFreedRelease() {
	if liveTrack.Load() {
		liveFreedUse.Add(1)
	}
}

// liveDropped counts an object leaving the heap; one made outside the tracking window is a stray, so it cannot cancel another object's leak.
func liveDropped(tracked bool) {
	if tracked {
		liveFreed.Add(1)
	} else {
		liveStray.Add(1)
	}
}

func (m *Machine) immortalize(v *obj) {
	if liveTrack.Load() && atomic.LoadInt32(&v.rc) > 0 {
		liveDropped(v.live)
	}
	setImmortal(v)
}

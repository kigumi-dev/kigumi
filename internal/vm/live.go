package vm

import "sync/atomic"

var (
	liveTrack                      atomic.Bool
	liveMade, liveFreed, liveStray atomic.Int64
)

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

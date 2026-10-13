package vm

import "sync/atomic"

var (
	liveTrack           atomic.Bool
	liveMade, liveFreed atomic.Int64
)

func (m *Machine) immortalize(v *obj) {
	if liveTrack.Load() && atomic.LoadInt32(&v.rc) > 0 {
		liveFreed.Add(1)
	}
	setImmortal(v)
}

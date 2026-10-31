package vm

import (
	"crypto/rand"
	"encoding/binary"
	"sync"

	"kigumi/internal/compiler/sem"
)

var (
	mapSeedOnce sync.Once
	mapSeed     int64
)

// One seed per process, drawn once even when machines race; every probe of every Map reads it.
func (m *Machine) registerMapSeed() {
	m.host["map.seed"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		mapSeedOnce.Do(func() {
			var buf [8]byte
			if _, err := rand.Read(buf[:]); err == nil {
				mapSeed = int64(binary.BigEndian.Uint64(buf[:]))
			}
			if mapSeed == 0 {
				mapSeed = 0x1e3779b97f4a7c15
			}
		})
		return mkInt(mapSeed, m.numKind(sem.TyU64))
	}
}

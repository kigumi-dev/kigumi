package fetch

import "encoding/binary"

var blake2bIV = [8]uint64{
	0x6a09e667f3bcc908, 0xbb67ae8584caa73b,
	0x3c6ef372fe94f82b, 0xa54ff53a5f1d36f1,
	0x510e527fade682d1, 0x9b05688c2b3e6c1f,
	0x1f83d9abfb41bd6b, 0x5be0cd19137e2179,
}

var blake2bSigma = [12][16]byte{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	{14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3},
	{11, 8, 12, 0, 5, 2, 15, 13, 10, 14, 3, 6, 7, 1, 9, 4},
	{7, 9, 3, 1, 13, 12, 11, 14, 2, 6, 5, 10, 4, 0, 15, 8},
	{9, 0, 5, 7, 2, 4, 10, 15, 14, 1, 11, 12, 6, 8, 3, 13},
	{2, 12, 6, 10, 0, 11, 8, 3, 4, 13, 7, 5, 15, 14, 1, 9},
	{12, 5, 1, 15, 14, 13, 4, 10, 0, 7, 6, 3, 9, 2, 8, 11},
	{13, 11, 7, 14, 12, 1, 3, 9, 5, 0, 15, 4, 8, 6, 2, 10},
	{6, 15, 14, 9, 11, 3, 0, 8, 12, 2, 13, 7, 1, 4, 10, 5},
	{10, 2, 8, 4, 7, 6, 1, 5, 15, 11, 9, 14, 3, 12, 13, 0},
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	{14, 10, 4, 8, 9, 15, 13, 6, 1, 12, 0, 2, 11, 7, 5, 3},
}

type blake2bState struct {
	h      [8]uint64
	t      [2]uint64
	buf    [128]byte
	buflen int
}

func newBlake2b512() *blake2bState {
	s := &blake2bState{h: blake2bIV}
	s.h[0] ^= 0x01010000 | 64 // param block: fanout=1, depth=1, digest length=64
	return s
}

func rotr64(x uint64, n uint) uint64 { return x>>n | x<<(64-n) }

func (s *blake2bState) compress(block *[128]byte, last bool) {
	var m [16]uint64
	for i := range m {
		m[i] = binary.LittleEndian.Uint64(block[i*8:])
	}
	v := [16]uint64{
		s.h[0], s.h[1], s.h[2], s.h[3], s.h[4], s.h[5], s.h[6], s.h[7],
		blake2bIV[0], blake2bIV[1], blake2bIV[2], blake2bIV[3],
		blake2bIV[4] ^ s.t[0], blake2bIV[5] ^ s.t[1], blake2bIV[6], blake2bIV[7],
	}
	if last {
		v[14] = ^v[14]
	}
	g := func(a, b, c, d, x, y int) {
		va, vb, vc, vd := v[a], v[b], v[c], v[d]
		va = va + vb + m[x]
		vd = rotr64(vd^va, 32)
		vc = vc + vd
		vb = rotr64(vb^vc, 24)
		va = va + vb + m[y]
		vd = rotr64(vd^va, 16)
		vc = vc + vd
		vb = rotr64(vb^vc, 63)
		v[a], v[b], v[c], v[d] = va, vb, vc, vd
	}
	for round := range 12 {
		sg := blake2bSigma[round]
		g(0, 4, 8, 12, int(sg[0]), int(sg[1]))
		g(1, 5, 9, 13, int(sg[2]), int(sg[3]))
		g(2, 6, 10, 14, int(sg[4]), int(sg[5]))
		g(3, 7, 11, 15, int(sg[6]), int(sg[7]))
		g(0, 5, 10, 15, int(sg[8]), int(sg[9]))
		g(1, 6, 11, 12, int(sg[10]), int(sg[11]))
		g(2, 7, 8, 13, int(sg[12]), int(sg[13]))
		g(3, 4, 9, 14, int(sg[14]), int(sg[15]))
	}
	for i := range 8 {
		s.h[i] ^= v[i] ^ v[i+8]
	}
}

func (s *blake2bState) write(p []byte) {
	for len(p) > 0 {
		if s.buflen == 128 {
			s.advanceCounter(128)
			s.compress(&s.buf, false)
			s.buflen = 0
		}
		n := copy(s.buf[s.buflen:], p)
		s.buflen += n
		p = p[n:]
	}
}

func (s *blake2bState) advanceCounter(n uint64) {
	s.t[0] += n
	if s.t[0] < n {
		s.t[1]++
	}
}

func (s *blake2bState) sum() [64]byte {
	s.advanceCounter(uint64(s.buflen))
	for i := s.buflen; i < 128; i++ {
		s.buf[i] = 0
	}
	s.compress(&s.buf, true)
	var out [64]byte
	for i, word := range s.h {
		binary.LittleEndian.PutUint64(out[i*8:], word)
	}
	return out
}

// Blake2b512Sum is the unkeyed BLAKE2b-512 digest minisign's "ED" signatures
// sign, not the file itself; exported for zigrelease.
func Blake2b512Sum(data []byte) [64]byte {
	s := newBlake2b512()
	s.write(data)
	return s.sum()
}

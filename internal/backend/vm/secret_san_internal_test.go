package vm

import "testing"

// TestSecretSanSelectShadow pins the select rule M4 asks for: a mask that is
// secret does not shadow a bit where both arms agree (a public zero in both
// arms clears it), while a bit where the arms differ follows the mask.
func TestSecretSanSelectShadow(t *testing.T) {
	m := &Machine{opts: Options{SecretSan: true}, sh: newShared()}
	pub := func(v int64) *obj {
		n := newObj(kInt)
		n.i, n.numk = v, 64
		return n
	}
	secretMask := pub(1)
	m.addLabel(secretMask, fullShadow(64))

	if got := m.selectShadow(secretMask, pub(0), pub(0)); got.tainted() {
		t.Errorf("select of two public zeros under a secret mask is shadowed: %+v", got)
	}
	if got := m.selectShadow(secretMask, pub(0), pub(1)); got.lo != 1 || got.hi != 0 {
		t.Errorf("select of arms differing only in bit 0: %+v, want only bit 0", got)
	}

	empty := shadow{}
	if got := m.selectShadow(pub(1), pub(0), pub(1)); got != empty {
		t.Errorf("public mask and arms are shadowed: %+v", got)
	}

	secretArm := pub(0)
	m.addLabel(secretArm, fullShadow(64))
	if got := m.selectShadow(pub(1), secretArm, pub(0)); got.lo != ^uint64(0) {
		t.Errorf("secret arm lost its label: %+v", got)
	}
}

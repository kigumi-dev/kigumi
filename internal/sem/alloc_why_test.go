package sem

import "testing"

func TestAllocReasonsAreDistinct(t *testing.T) {
	seen := map[uint8]string{}
	for name, why := range map[string]uint8{
		"build": whyBuild, "sharedValue": whySharedValue, "directCross": whyDirectCross,
		"inlined": whyInlined, "genericCross": whyGenericCross, "copy": whyCopy,
	} {
		if other, dup := seen[why]; dup {
			t.Errorf("%s and %s share reason %d", name, other, why)
		}
		seen[why] = name
	}
}

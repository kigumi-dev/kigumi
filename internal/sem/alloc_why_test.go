package sem

import "testing"

func TestAllocReasonsAreDistinct(t *testing.T) {
	seen := map[uint8]string{}
	for name, why := range map[string]uint8{
		"build": whyBuild, "sharedValue": whySharedValue, "directCross": whyDirectCross,
		"inlined": whyInlined, "genericCross": whyGenericCross, "copy": whyCopy,
		"copyHeld": whyCopyHeld, "copyElem": whyCopyElem, "copyBox": whyCopyBox, "copyInst": whyCopyInst, "copyContent": whyCopyContent, "bind": whyBind,
		"entryBox": whyEntryBox, "dynamicResult": whyDynamicResult, "witnessCross": whyWitnessCross, "witnessResult": whyWitnessResult, "boxedArg": whyBoxedArg, "witnessBind": whyWitnessBind,
	} {
		if other, dup := seen[why]; dup {
			t.Errorf("%s and %s share reason %d", name, other, why)
		}
		seen[why] = name
	}
}

package driver_test

import "testing"

func TestBuildFreestandingDirectStatics(t *testing.T) {
	src := "static mut ticks: u32 = 0\nstatic mut level: Int = 3\nstatic mut armed: Bool = false\n\n" +
		"export(interrupt) {\n    pub fn tick() {\n        unsafe {\n            ticks = ticks.wrappingAdd(1)\n" +
		"            level = level.wrappingAdd(2)\n            armed = level > 4\n        }\n    }\n}\n\n" +
		"noalloc fn bump() {\n    unsafe {\n        ticks = ticks.wrappingAdd(1)\n    }\n}\n\n" +
		"bump()\ntick()\nunsafe {\n    print \"${ticks} ${level} ${armed}\"\n}\n"
	checkFreestandingSymbols(t, "x86_64-freestanding", src)
}

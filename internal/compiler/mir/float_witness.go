package mir

func FloatWitnessKey(key string) bool {
	switch key {
	case "prelude.f64.equals", "prelude.f64.compareTo", "prelude.f32.equals", "prelude.f32.compareTo":
		return true
	}
	return false
}

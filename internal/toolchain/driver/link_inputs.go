package driver

// linkInputs: with pinCompDir, out becomes absolute and inputs relative to
// dir, because cc runs with cmd.Dir = dir while inputs may be cwd-relative.
func linkInputs(o CompilerOptions, pinCompDir bool, dir, out string, inputs []string) (string, []string, error) {
	if !pinCompDir {
		return out, inputs, nil
	}
	abs, err := o.Abs(out)
	if err != nil {
		return "", nil, err
	}
	rel := make([]string, 0, len(inputs))
	for _, in := range inputs {
		r, err := pinnedInput(o, dir, in)
		if err != nil {
			return "", nil, err
		}
		rel = append(rel, r)
	}
	return abs, rel, nil
}

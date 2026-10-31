package driver

// cwdFor is the directory a run uses: the run's own choice, else the options'.
func (m *Module) cwdFor(own string) string {
	if own != "" {
		return own
	}
	return m.compiler.Cwd
}

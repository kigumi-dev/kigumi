package sem

func moduleRank(m string) int {
	switch m {
	case "":
		return 0
	case "std":
		return 2
	}
	return 1
}

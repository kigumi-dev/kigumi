package ops

var callbackArgs = func() map[StdOpId]int {
	out := map[StdOpId]int{}
	for _, op := range Registry.Ops {
		if op.Callback >= 0 {
			out[StdOpId(op.ID)] = op.Callback
		}
	}
	return out
}()

func CallbackArg(id StdOpId) (int, bool) { index, ok := callbackArgs[id]; return index, ok }

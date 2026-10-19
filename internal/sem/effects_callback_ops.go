package sem

// callbackOpSummaries lists only the wrapper's own facts; the callback's facts merge in at each call site.
var callbackOpSummaries = map[string]OpSummary{
	"alloc.Shared.with":          ownFacts(FactAlloc, FactPanic),
	"alloc.Shared.withMut":       ownFacts(FactAlloc, FactPanic),
	"array.Array.with":           ownFacts(FactAlloc),
	"array.Array.withMut":        ownFacts(FactAlloc),
	"prelude.FixedArray.withMut": ownFacts(),
	"array.Array.tryMap":         ownFacts(FactAlloc),
	"array.Array.tryFilter":      ownFacts(FactAlloc),
	"array.trySortedBy":          ownFacts(FactAlloc),
	"ffi.Pin.with":               ownFacts(FactAlloc),
	"ffi.HandleTable.with":       ownFacts(FactAlloc, FactPanic),
	"ffi.withAddress":            ownFacts(FactAlloc),
	"ffi.withAddressConst":       ownFacts(FactAlloc),
}

func ownFacts(facts ...OpFact) OpSummary {
	s := OpSummary{Known: true}
	for _, f := range facts {
		s.Facts[f] = OpFactState{Maybe: true}
	}
	return s
}

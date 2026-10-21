package sem

func (c *checker) settlePendingReceiver(recv, valueType TypeID, name string) (TypeID, TypeID) {
	i, ok := c.vars.pendingIndex(valueType)
	if !ok {
		return recv, valueType
	}
	untyped := TyUntypedInt
	if c.vars.pending[i] == 2 {
		untyped = TyUntypedFloat
	}
	if resolved, ok := c.resolveLiteralReceiver(untyped, name); ok && c.vars.unify(valueType, resolved) {
		return c.vars.resolve(recv), c.vars.resolve(valueType)
	}
	return recv, valueType
}

package sem

// requestProtoMethod asks for the derived `equals` / `hash` of t when a call names it
// directly, as `==` and a Map key do; the first pass reports nothing.
func (c *checker) requestProtoMethod(t TypeID, name string, args int) bool {
	proto := ""
	switch {
	case name == "equals" && args == 1:
		proto = "Eq"
	case name == "hash" && args == 0:
		proto = "Hash"
	default:
		return false
	}
	iface := c.r.langItem(0, 0, proto)
	return iface != 0 && c.r.derivable(t, proto) && c.r.requestDerive(t, c.r.Types.Iface(iface, nil))
}

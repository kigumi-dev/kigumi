package vm

func (m *Machine) partialRecord(v *obj) bool {
	stack := []freeFrame{{v: v, fields: v.fields, borrow: v.k == kRecord, step: 1}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		child, ok := top.nextChild(m)
		if !ok {
			stack = stack[:len(stack)-1]
			continue
		}
		if child == nil {
			return true
		}
		if child.k == kRecord || child.k == kVariant {
			stack = append(stack, freeFrame{v: child, fields: child.fields, borrow: child.k == kRecord, step: 1})
		}
	}
	return false
}

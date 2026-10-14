package mir

import "kigumi/internal/sem"

func (p *Program) ReceiverMismatch(owner, got sem.TypeID) string {
	return p.receiverMismatch(owner, got)
}

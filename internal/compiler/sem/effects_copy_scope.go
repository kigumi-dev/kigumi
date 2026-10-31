package sem

import "kigumi/internal/compiler/syntax"

type allocBlock struct {
	file FileID
	body syntax.NodeID
}

// InAllocatorBlock reports whether node sits inside the body of an `allocator` expression, as the checker's Scoped edges do.
func (r *Result) InAllocatorBlock(file FileID, node syntax.NodeID) bool {
	if node == 0 {
		return false
	}
	t := r.Tree(file)
	at := t.Span(node)
	for _, b := range r.allocBlocks {
		if b.file != file {
			continue
		}
		if body := t.Span(b.body); body.Start <= at.Start && at.End <= body.End {
			return true
		}
	}
	return false
}

func (r *Result) recheckProfiles(nodes []EntityID) {
	r.inferDefault(nodes)
	if r.noDefault {
		r.checkDefaultAllocator()
	}
	if r.recoverableOnly {
		r.checkRecoverableOnly(nodes)
	}
}

package mir

type verifiedFlow struct {
	hash uint64
	weak []bool
}

type verificationHistory map[*Func]verifiedFlow

func (p *verifier) rememberFlow(f *Func, hash uint64) {
	if p.verified == nil {
		p.verified = verificationHistory{}
	}
	p.verified[f] = verifiedFlow{hash: hash, weak: p.body(f).weakDefs}
}

func (p *verifier) pruneHistory(fs []*Func) {
	if len(p.verified) == 0 {
		return
	}
	live := make(map[*Func]bool, len(fs))
	for _, f := range fs {
		live[f] = true
	}
	for f := range p.verified {
		if !live[f] {
			delete(p.verified, f)
		}
	}
}

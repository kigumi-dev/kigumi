package main

import (
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strings"
)

func qeBody(quote []byte) []byte {
	aux := quote[436+128:]
	if binary.LittleEndian.Uint16(aux) == 6 {
		aux = aux[6:]
	}
	return append([]byte(nil), aux[:384]...)
}

func certification(kind uint16, value []byte) []byte {
	out := make([]byte, 6)
	binary.LittleEndian.PutUint16(out, kind)
	binary.LittleEndian.PutUint32(out[2:], uint32(len(value)))
	return append(out, value...)
}

func regenerate(v *vector, m material, base []byte) {
	if v.Op == "sgx-tcb" {
		return
	}
	if v.Op == "sgx-pck" {
		chain, root := m.chain, m.root
		switch v.ID {
		case "wrong-anchor":
			root = m.wrongRoot
		case "p384-chain":
			chain = m.p384Chain
		case "p384-anchor":
			root = m.p384Root
		case "bad-leaf-signature":
			bad := append([]byte(nil), m.leafDer...)
			bad[len(bad)-1] ^= 1
			chain = append(pemCert(bad), m.intermediatePem...)
		}
		v.Hex, v.Anchors = hex.EncodeToString(chain), hex.EncodeToString(root)
		return
	}
	old := decode(v.Hex)
	if len(old) < 432 {
		return
	}
	body := append([]byte(nil), old[:432]...)
	public := elliptic.Marshal(elliptic.P256(), m.attest.X, m.attest.Y)[1:]
	quoteSig := sign(m.attest, body)
	if v.ID == "forged-quote" || v.Op == "sgx-signature" && v.ID == "forged" {
		quoteSig[0] ^= 1
	}
	report := qeBody(base)
	auth := []byte("kigumi SGX fixture authentication")
	digest := sha256.Sum256(append(append([]byte(nil), public...), auth...))
	clear(report[320:])
	copy(report[320:], digest[:])
	if v.ID == "wrong-binding" {
		report[320] ^= 1
	}
	qeSig := sign(m.leaf, report)
	if v.ID == "forged-qe" || v.Op == "sgx-qe" && v.ID == "forged" {
		qeSig[0] ^= 1
	}
	chain := m.chain
	if v.ID == "p384-chain" {
		chain = m.p384Chain
	}
	aux := append(append(append([]byte(nil), quoteSig...), public...), report...)
	aux = append(aux, qeSig...)
	aux = binary.LittleEndian.AppendUint16(aux, uint16(len(auth)))
	aux = append(aux, auth...)
	cert := certification(5, chain)
	if v.ID == "unknown-cert" {
		cert = certification(7, []byte("unknown certification"))
	}
	if v.ID == "bad-cert-length" {
		binary.LittleEndian.PutUint32(cert[2:], uint32(len(chain)-1))
	}
	aux = append(aux, cert...)
	if v.ID == "type5" {
		aux = append(append(append([]byte(nil), quoteSig...), public...), certification(5, chain)...)
		v.Expected = "error:bad-certification-data"
	}
	quote := binary.LittleEndian.AppendUint32(body, uint32(len(aux)))
	quote = append(quote, aux...)
	if v.ID == "truncated" {
		quote = quote[:len(quote)-1]
	} else if v.ID == "trailing" {
		quote = append(quote, 0)
	}
	v.Hex = hex.EncodeToString(quote)
	if v.Anchors != "" {
		root := m.root
		if strings.Contains(v.ID, "wrong-anchor") {
			root = m.wrongRoot
		}
		v.Anchors = hex.EncodeToString(root)
	}
}

package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

var (
	intelQeVendor = decode("939a7233f79c4ca9940a0db3957f0607")
	qeMrSigner    = decode("8c4f5775d796503e96137f77c68a829a0056ac8ded70140b081b094490c57bff")
	enclaveMr     = bytes.Repeat([]byte{0x11}, 32)
	enclaveSigner = bytes.Repeat([]byte{0x22}, 32)
	enclaveNonce  = bytes.Repeat([]byte{0x33}, 64)
)

// reportBody lays out a 384-byte sgx_report_body_t.
type reportBody struct {
	cpuSvn     []byte
	misc       uint32
	attributes []byte
	mrEnclave  []byte
	mrSigner   []byte
	isvProdID  uint16
	isvSvn     uint16
	reportData []byte
}

func (r reportBody) bytes() []byte {
	out := make([]byte, 384)
	copy(out[0x00:], r.cpuSvn)
	binary.LittleEndian.PutUint32(out[0x10:], r.misc)
	copy(out[0x30:], r.attributes)
	copy(out[0x40:], r.mrEnclave)
	copy(out[0x80:], r.mrSigner)
	binary.LittleEndian.PutUint16(out[0x100:], r.isvProdID)
	binary.LittleEndian.PutUint16(out[0x102:], r.isvSvn)
	copy(out[0x140:], r.reportData)
	return out
}

type quoteSpec struct {
	chain        []byte
	pck          *ecdsa.PrivateKey
	qeSvn        uint16
	qeSigner     []byte
	qeProdID     uint16
	vendor       []byte
	debug        bool
	forgeQuote   bool
	forgeQe      bool
	breakBinding bool
}

// The quote's own CPUSVN and PCE SVN are maximal, so a verifier that reads
// them instead of the PCK certificate's TCB selects the wrong level.
func collateralQuote(attest *ecdsa.PrivateKey, spec quoteSpec) []byte {
	header := make([]byte, 48)
	binary.LittleEndian.PutUint16(header[0:], 3)
	binary.LittleEndian.PutUint16(header[2:], 2)
	binary.LittleEndian.PutUint16(header[8:], 8)
	binary.LittleEndian.PutUint16(header[10:], 0xff)
	copy(header[12:], intelQeVendor)
	if spec.vendor != nil {
		copy(header[12:], spec.vendor)
	}
	attributes := []byte{0x05}
	if spec.debug {
		attributes = []byte{0x07}
	}
	enclave := reportBody{
		cpuSvn: bytes.Repeat([]byte{0xff}, 16), attributes: attributes, mrEnclave: enclaveMr, mrSigner: enclaveSigner,
		isvProdID: 1, isvSvn: 2, reportData: enclaveNonce,
	}
	body := append(header, enclave.bytes()...)
	quoteSig := sign(attest, body)
	if spec.forgeQuote {
		quoteSig[0] ^= 1
	}
	public := elliptic.Marshal(elliptic.P256(), attest.X, attest.Y)[1:]
	auth := []byte("kigumi SGX collateral fixture")
	digest := sha256.Sum256(append(append([]byte(nil), public...), auth...))
	signer, prodID := spec.qeSigner, spec.qeProdID
	if signer == nil {
		signer, prodID = qeMrSigner, 1
	}
	qe := reportBody{
		cpuSvn: make([]byte, 16), attributes: decode("15000000000000000700000000000000"), mrEnclave: bytes.Repeat([]byte{0x55}, 32),
		mrSigner: signer, isvProdID: prodID, isvSvn: spec.qeSvn, reportData: append(digest[:], make([]byte, 32)...),
	}.bytes()
	if spec.breakBinding {
		qe[0x140] ^= 1
	}
	qeSig := sign(spec.pck, qe)
	if spec.forgeQe {
		qeSig[0] ^= 1
	}
	aux := append(append(append(quoteSig, public...), qe...), qeSig...)
	aux = binary.LittleEndian.AppendUint16(aux, uint16(len(auth)))
	aux = append(aux, auth...)
	aux = append(aux, certification(5, spec.chain)...)
	quote := binary.LittleEndian.AppendUint32(body, uint32(len(aux)))
	return append(quote, aux...)
}

func hexOf(b []byte) string { return hex.EncodeToString(b) }

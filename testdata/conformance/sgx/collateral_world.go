package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

// Common names carry the phrase Intel QVL requires of each role.
const (
	rootName   = "Kigumi Fixture SGX Root CA"
	caName     = "Kigumi Fixture SGX PCK Platform CA"
	pckName    = "Kigumi Fixture SGX PCK Certificate"
	signerName = "Kigumi Fixture SGX TCB Signing"
)

var (
	verifyAt    = date(2025, time.June, 1)
	issuedAt    = date(2025, time.May, 25)
	nextUpdate  = date(2025, time.June, 24)
	crlThis     = date(2025, time.May, 30)
	crlNext     = date(2025, time.June, 29)
	fixtureFmsp = decode("00906ed50000")
	fixturePce  = decode("0000")
)

type world struct {
	root, otherRoot, ca, signer, otherSigner *authority
	attest                                   *ecdsa.PrivateKey
	pckKey                                   *ecdsa.PrivateKey
	nextSerial                               int64
}

func newWorld() *world {
	w := &world{nextSerial: 100}
	w.root = issue(certSpec{serial: 1, name: rootName, key: "collateral root", ca: true, pathLen: 1, notBefore: date(2020, 1, 1), notAfter: date(2049, 12, 31)}, nil)
	w.otherRoot = issue(certSpec{serial: 1, name: rootName, key: "collateral other root", ca: true, pathLen: 1, notBefore: date(2020, 1, 1), notAfter: date(2049, 12, 31)}, nil)
	w.ca = issue(certSpec{serial: 2, name: caName, key: "collateral pck ca", ca: true, pathLen: 0, notBefore: date(2020, 1, 1), notAfter: date(2040, 1, 1)}, w.root)
	w.signer = issue(certSpec{serial: 3, name: signerName, key: "collateral tcb signer", notBefore: date(2024, 1, 1), notAfter: date(2030, 1, 1)}, w.root)
	w.otherSigner = issue(certSpec{serial: 3, name: signerName, key: "collateral other signer", notBefore: date(2024, 1, 1), notAfter: date(2030, 1, 1)}, w.otherRoot)
	w.attest = fixedKey("collateral attestation", elliptic.P256())
	w.pckKey = fixedKey("collateral pck", elliptic.P256())
	return w
}

// pck issues a PCK certificate for `tcb`; each call gets a fresh serial so a
// CRL can revoke exactly one of them.
func (w *world) pck(tcb platformTcb, edit sgxExtensionEdit, withExtension bool) *authority {
	w.nextSerial++
	var extra []pkix.Extension
	if withExtension {
		extra = []pkix.Extension{{Id: oidSgxExtension, Value: sgxExtension(tcb, edit)}}
	}
	return issue(certSpec{serial: w.nextSerial, name: pckName, key: "collateral pck", notBefore: date(2024, 1, 1), notAfter: date(2031, 1, 1), extra: extra}, w.ca)
}

func (w *world) chain(leaf *authority, withRoot bool) []byte {
	out := append(pemCert(leaf.der), pemCert(w.ca.der)...)
	if withRoot {
		out = append(out, pemCert(w.root.der)...)
	}
	return out
}

func (w *world) signingChain(signer, root *authority) []byte {
	return append(pemCert(signer.der), pemCert(root.der)...)
}

func (w *world) pckCrl(revoked ...*authority) []byte {
	return crl(w.ca, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, revoked: revocations(revoked)})
}

func (w *world) rootCrl(revoked ...*authority) []byte {
	return crl(w.root, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, revoked: revocations(revoked)})
}

func revocations(certs []*authority) []revocation {
	var out []revocation
	for _, cert := range certs {
		out = append(out, revocation{serial: new(big.Int).Set(cert.cert.SerialNumber), reason: 1})
	}
	return out
}

func svns(values ...byte) [16]byte {
	var out [16]byte
	copy(out[:], values)
	return out
}

func fixtureTcb(components [16]byte, pceSvn int) platformTcb {
	return platformTcb{components: components, pceSvn: pceSvn, fmspc: fixtureFmsp, pceID: fixturePce}
}

// The TCB Info levels, listed out of QVL's order so a verifier that takes
// the first reachable level in document order picks the wrong one.
func fixtureLevels() []level {
	d := date(2025, time.February, 12)
	old := date(2024, time.August, 14)
	return []level{
		{components: svns(1, 1, 1, 1, 1, 1, 0, 1), pceSvn: 9, status: "Revoked", date: date(2022, 3, 9), advisories: []string{"INTEL-SA-00219"}},
		{components: svns(2, 2, 2, 2, 3, 1, 0, 2), pceSvn: 10, status: "OutOfDateConfigurationNeeded", date: old, advisories: []string{"INTEL-SA-00615", "INTEL-SA-00657"}},
		{components: svns(2, 2, 2, 2, 3, 1, 0, 3), pceSvn: 10, status: "OutOfDate", date: old, advisories: []string{"INTEL-SA-00615"}},
		{components: svns(3, 3, 3, 3, 4, 1, 0, 3), pceSvn: 11, status: "ConfigurationAndSWHardeningNeeded", date: d, advisories: []string{"INTEL-SA-00161", "INTEL-SA-00233"}},
		{components: svns(4, 4, 3, 3, 4, 1, 0, 3), pceSvn: 13, status: "UpToDate", date: d},
		{components: svns(3, 3, 3, 3, 4, 1, 0, 3), pceSvn: 13, status: "ConfigurationNeeded", date: d, advisories: []string{"INTEL-SA-00161"}},
		{components: svns(4, 4, 3, 3, 4, 1, 0, 3), pceSvn: 11, status: "SWHardeningNeeded", date: d, advisories: []string{"INTEL-SA-00334"}},
	}
}

// The PCK TCBs that select each level above, plus one below every level.
var platformFor = map[string]platformTcb{
	"up-to-date":                fixtureTcb(svns(4, 4, 3, 3, 4, 1, 0, 3), 13),
	"sw-hardening":              fixtureTcb(svns(4, 4, 3, 3, 4, 1, 0, 3), 12),
	"configuration":             fixtureTcb(svns(3, 3, 3, 3, 4, 1, 0, 3), 13),
	"configuration-and-sw":      fixtureTcb(svns(3, 3, 3, 3, 4, 1, 0, 3), 11),
	"out-of-date":               fixtureTcb(svns(2, 2, 2, 2, 3, 1, 0, 3), 10),
	"out-of-date-configuration": fixtureTcb(svns(2, 2, 2, 2, 3, 1, 0, 2), 10),
	"revoked":                   fixtureTcb(svns(1, 1, 1, 1, 1, 1, 0, 1), 9),
	"not-supported":             fixtureTcb(svns(1, 1, 1, 1, 1, 1, 0, 1), 8),
	"high-components-low-pce":   fixtureTcb(svns(9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9), 10),
	"one-component-below":       fixtureTcb(svns(4, 4, 3, 2, 4, 1, 0, 3), 13),
}

type qeLevel struct {
	isvSvn     int
	status     string
	date       time.Time
	advisories []string
}

func fixtureQeLevels() []qeLevel {
	return []qeLevel{
		{isvSvn: 6, status: "OutOfDate", date: date(2024, time.March, 13), advisories: []string{"INTEL-SA-00477"}},
		{isvSvn: 8, status: "UpToDate", date: date(2025, time.February, 12)},
		{isvSvn: 3, status: "Revoked", date: date(2023, time.February, 15), advisories: []string{"INTEL-SA-00202"}},
	}
}

func qeIdentityBody(id string, issued, next time.Time, evaluation int, levels []qeLevel) qeIdentityJSON {
	doc := qeIdentityJSON{
		ID: id, Version: 2, IssueDate: issued.Format(pcsTime), NextUpdate: next.Format(pcsTime), TcbEvaluationDataNumber: evaluation,
		Miscselect: "00000000", MiscselectMask: "FFFFFFFF", Attributes: "11000000000000000000000000000000",
		AttributesMask: "FBFFFFFFFFFFFFFF0000000000000000", Mrsigner: "8C4F5775D796503E96137F77C68A829A0056AC8DED70140B081B094490C57BFF", IsvProdID: 1,
	}
	for _, l := range levels {
		doc.TcbLevels = append(doc.TcbLevels, qeLevelJSON{Tcb: qeTcbJSON{IsvSvn: l.isvSvn}, TcbDate: l.date.Format(pcsTime), TcbStatus: l.status, AdvisoryID: l.advisories})
	}
	return doc
}

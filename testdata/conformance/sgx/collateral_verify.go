package main

import (
	"bytes"
	"crypto/x509/pkix"
	"encoding/hex"
	"maps"
	"math/big"
	"slices"
	"strings"
	"time"
)

type collateralRef struct {
	TcbInfo    string `json:"tcbInfo,omitempty"`
	TcbChain   string `json:"tcbChain,omitempty"`
	QeIdentity string `json:"qeIdentity,omitempty"`
	QeChain    string `json:"qeChain,omitempty"`
	PckCrl     string `json:"pckCrl,omitempty"`
	RootCrl    string `json:"rootCrl,omitempty"`
}

func goodCollateral() *collateralRef {
	return &collateralRef{TcbInfo: "tcb", TcbChain: "chain", QeIdentity: "qe", QeChain: "chain", PckCrl: "pck-crl", RootCrl: "root-crl"}
}

func (c collateralRef) with(edit func(*collateralRef)) *collateralRef {
	edit(&c)
	return &c
}

type blobs map[string]string

func (b blobs) put(name string, value []byte) { b[name] = hex.EncodeToString(value) }

func tamperStatus(raw []byte) []byte {
	return bytes.Replace(raw, []byte(`"tcbStatus":"Revoked"`), []byte(`"tcbStatus":"UpToDate"`), 1)
}

func millis(t time.Time) int64 { return t.UnixMilli() }

// verifySuite uses one quote per PCK TCB, so each status comes from the
// level the PCK certificate selects.
func verifySuite(w *world) suite {
	file := suite{Primitive: "sgx-verify", Blobs: blobs{},
		Source: "End-to-end production DCAP verification against a synthetic Intel-shaped P-256 hierarchy: TCB Info v3 and QE Identity v2 signed by a TCB signing certificate, PCK and root CA CRLs, statuses per Intel QVL sgxEvaluateTCB"}
	b := blobs(file.Blobs)
	levels := fixtureLevels()
	qeLevels := fixtureQeLevels()
	b.put("tcb", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, issuedAt, nextUpdate, 17, levels), w.signer.key, nil))
	b.put("tcb-other-fmspc", signedDocument("tcbInfo", tcbInfoBody(decode("00906ed50001"), fixturePce, issuedAt, nextUpdate, 17, levels), w.signer.key, nil))
	b.put("tcb-expired", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, date(2025, 4, 1), date(2025, 5, 1), 16, levels), w.signer.key, nil))
	b.put("tcb-tampered", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, issuedAt, nextUpdate, 17, levels), w.signer.key, tamperStatus))
	b.put("tcb-foreign", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, issuedAt, nextUpdate, 17, levels), w.otherSigner.key, nil))
	b.put("qe", signedDocument("enclaveIdentity", qeIdentityBody("QE", issuedAt, nextUpdate, 17, qeLevels), w.signer.key, nil))
	b.put("qe-future", signedDocument("enclaveIdentity", qeIdentityBody("QE", date(2025, 6, 10), date(2025, 7, 10), 18, qeLevels), w.signer.key, nil))
	b.put("qe-older", signedDocument("enclaveIdentity", qeIdentityBody("QE", issuedAt, nextUpdate, 16, qeLevels), w.signer.key, nil))
	b.put("tcb-issued-after-now", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, verifyAt.Add(time.Second), nextUpdate, 17, levels), w.signer.key, nil))
	b.put("chain", w.signingChain(w.signer, w.root))
	b.put("chain-foreign", w.signingChain(w.otherSigner, w.otherRoot))
	leaves := map[string]*authority{}
	for _, name := range slices.Sorted(maps.Keys(platformFor)) {
		leaves[name] = w.pck(platformFor[name], sgxExtensionEdit{}, true)
	}
	noExtension := w.pck(platformFor["up-to-date"], sgxExtensionEdit{}, false)
	b.put("pck-crl", crl(w.ca, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, revoked: []revocation{{serial: bigInt(999), reason: 1}}}))
	b.put("pck-crl-revokes-leaf", w.pckCrl(leaves["up-to-date"]))
	b.put("pck-crl-expired", crl(w.ca, crlSpec{thisUpdate: date(2025, 5, 1), nextUpdate: date(2025, 5, 31)}))
	b.put("pck-crl-by-root", crl(w.root, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext}))
	b.put("root-crl", w.rootCrl())
	b.put("root-crl-revokes-ca", w.rootCrl(w.ca))
	b.put("root-crl-revokes-signer", w.rootCrl(w.signer))
	anchors := hex.EncodeToString(pemCert(w.root.der))
	quotes := map[string]string{}
	quote := func(leaf *authority, qeSvn uint16, edit func(*quoteSpec)) string {
		spec := quoteSpec{chain: w.chain(leaf, true), pck: w.pckKey, qeSvn: qeSvn}
		if edit != nil {
			edit(&spec)
		}
		return hex.EncodeToString(collateralQuote(w.attest, spec))
	}
	add := func(id, quoteHex string, collateral *collateralRef, accept string, expected string) *vector {
		name, ok := quotes[quoteHex]
		if !ok {
			name = "quote-" + id
			quotes[quoteHex] = name
			b[name] = quoteHex
		}
		file.Vectors = append(file.Vectors, vector{ID: id, Op: "sgx-verify", Quote: name, Anchors: anchors, Now: millis(verifyAt),
			ReportData: hex.EncodeToString(enclaveNonce), Measurement: hex.EncodeToString(enclaveMr), Collateral: collateral, Accept: accept, Expected: expected})
		return &file.Vectors[len(file.Vectors)-1]
	}
	good := goodCollateral()
	up := quote(leaves["up-to-date"], 8, nil)
	add("good", up, good, "", goodResult)
	for _, c := range []struct{ name, expected string }{
		{"sw-hardening", "status=sw-hardening-needed platform=sw-hardening-needed qe=up-to-date date=2025-02-12T00:00:00Z advisories=INTEL-SA-00334"},
		{"configuration", "status=configuration-needed platform=configuration-needed qe=up-to-date date=2025-02-12T00:00:00Z advisories=INTEL-SA-00161"},
		{"configuration-and-sw", "status=configuration-and-sw-hardening-needed platform=configuration-and-sw-hardening-needed qe=up-to-date date=2025-02-12T00:00:00Z advisories=INTEL-SA-00161,INTEL-SA-00233"},
		{"out-of-date", "status=out-of-date platform=out-of-date qe=up-to-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615"},
		{"out-of-date-configuration", "status=out-of-date-configuration-needed platform=out-of-date-configuration-needed qe=up-to-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615,INTEL-SA-00657"},
		{"high-components-low-pce", "status=out-of-date platform=out-of-date qe=up-to-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615"},
	} {
		add("status-"+c.name, quote(leaves[c.name], 8, nil), good, "all", c.expected)
	}
	add("qe-out-of-date", quote(leaves["up-to-date"], 7, nil), good, "all", "status=out-of-date platform=up-to-date qe=out-of-date date=2024-03-13T00:00:00Z advisories=INTEL-SA-00477")
	add("qe-out-of-date-configuration", quote(leaves["configuration"], 6, nil), good, "all", "status=out-of-date-configuration-needed platform=configuration-needed qe=out-of-date date=2024-03-13T00:00:00Z advisories=INTEL-SA-00161,INTEL-SA-00477")
	add("status-revoked", quote(leaves["revoked"], 8, nil), good, "all", "error:tcb-revoked")
	add("qe-revoked", quote(leaves["up-to-date"], 5, nil), good, "all", "error:tcb-revoked")
	add("default-rejects-out-of-date", quote(leaves["out-of-date"], 8, nil), good, "", "error:status-rejected")
	add("tcb-not-supported", quote(leaves["not-supported"], 8, nil), good, "all", "error:tcb-not-supported")
	add("one-component-below", quote(leaves["one-component-below"], 8, nil), good, "all", "status=out-of-date platform=out-of-date qe=up-to-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615")
	add("qe-not-supported", quote(leaves["up-to-date"], 2, nil), good, "all", "error:tcb-not-supported")
	add("unevaluated", up, nil, "all", "error:missing-collateral")
	add("unevaluated-default", up, nil, "", "error:missing-collateral")
	add("fmspc-mismatch", up, good.with(func(c *collateralRef) { c.TcbInfo = "tcb-other-fmspc" }), "all", "error:tcb-info-mismatch")
	add("tcb-info-expired", up, good.with(func(c *collateralRef) { c.TcbInfo = "tcb-expired" }), "all", "error:collateral-expired")
	add("tcb-info-tampered", up, good.with(func(c *collateralRef) { c.TcbInfo = "tcb-tampered" }), "all", "error:collateral-signature-invalid")
	add("tcb-info-foreign-signer", up, good.with(func(c *collateralRef) { c.TcbInfo = "tcb-foreign" }), "all", "error:collateral-signature-invalid")
	add("tcb-chain-foreign-root", up, good.with(func(c *collateralRef) { c.TcbChain = "chain-foreign"; c.TcbInfo = "tcb-foreign" }), "all", "error:bad-signing-chain")
	add("qe-identity-issued-after-now", up, good.with(func(c *collateralRef) { c.QeIdentity = "qe-future" }), "", goodResult)
	add("tcb-info-issued-after-now", up, good.with(func(c *collateralRef) { c.TcbInfo = "tcb-issued-after-now" }), "", goodResult)
	add("minimum-evaluation-equal", up, good, "", goodResult).MinEvaluation = 17
	add("minimum-evaluation-qe-below", up, good.with(func(c *collateralRef) { c.QeIdentity = "qe-older" }), "", "error:policy-rejected").MinEvaluation = 17
	rootLeaf := issue(certSpec{serial: 90, name: pckName, key: "collateral pck", notBefore: date(2024, 1, 1), notAfter: date(2031, 1, 1),
		extra: []pkix.Extension{{Id: oidSgxExtension, Value: sgxExtension(platformFor["up-to-date"], sgxExtensionEdit{})}}}, w.root)
	direct := quote(rootLeaf, 8, func(s *quoteSpec) { s.chain = append(pemCert(rootLeaf.der), pemCert(w.root.der)...) })
	add("pck-leaf-issued-by-root", direct, good.with(func(c *collateralRef) { c.PckCrl = "root-crl" }), "all", "error:bad-pck-chain")
	add("pck-revoked", up, good.with(func(c *collateralRef) { c.PckCrl = "pck-crl-revokes-leaf" }), "all", "error:certificate-revoked")
	add("pck-ca-revoked", up, good.with(func(c *collateralRef) { c.RootCrl = "root-crl-revokes-ca" }), "all", "error:certificate-revoked")
	add("tcb-signer-revoked", up, good.with(func(c *collateralRef) { c.RootCrl = "root-crl-revokes-signer" }), "all", "error:certificate-revoked")
	add("pck-crl-expired", up, good.with(func(c *collateralRef) { c.PckCrl = "pck-crl-expired" }), "all", "error:collateral-expired")
	add("pck-crl-wrong-issuer", up, good.with(func(c *collateralRef) { c.PckCrl = "pck-crl-by-root" }), "all", "error:bad-crl")
	add("no-sgx-extension", quote(noExtension, 8, nil), good, "all", "error:bad-pck-extension")
	add("wrong-qe-vendor", quote(leaves["up-to-date"], 8, func(s *quoteSpec) { s.vendor = make([]byte, 16) }), good, "", "error:policy-rejected")
	add("wrong-binding", quote(leaves["up-to-date"], 8, func(s *quoteSpec) { s.breakBinding = true }), good, "", "error:qe-report-mismatch")
	truncated := decode(up)
	add("truncated", hex.EncodeToString(truncated[:len(truncated)-1]), good, "", "error:bad-length")
	add("trailing", up+"00", good, "", "error:bad-length")
	unified := append([]byte(nil), truncated...)
	unified[0] = 4
	add("unified-v4", hex.EncodeToString(unified), good, "", "error:unsupported-version")
	add("missing-anchor", up, good, "", "error:missing-trust-anchor").Anchors = ""
	add("forged-quote", quote(leaves["up-to-date"], 8, func(s *quoteSpec) { s.forgeQuote = true }), good, "", "error:signature-invalid")
	add("forged-qe", quote(leaves["up-to-date"], 8, func(s *quoteSpec) { s.forgeQe = true }), good, "", "error:signature-invalid")
	add("debug", quote(leaves["up-to-date"], 8, func(s *quoteSpec) { s.debug = true }), good, "", "error:policy-rejected")
	add("wrong-anchor", up, good, "", "error:pck-chain-mismatch").Anchors = hex.EncodeToString(pemCert(w.otherRoot.der))
	add("expired-pck", up, good, "", "error:bad-pck-chain").Now = millis(date(2031, 6, 1))
	add("nonce-mismatch", up, good, "", "error:policy-rejected").ReportData = strings.Repeat("44", 64)
	add("measurement-mismatch", up, good, "", "error:policy-rejected").Measurement = strings.Repeat("99", 32)
	add("required-fmspc", up, good, "", "error:policy-rejected").Hex2 = "00906ed50001"
	add("minimum-evaluation", up, good, "", "error:policy-rejected").MinEvaluation = 18
	add("chain-without-root", quote(leaves["up-to-date"], 8, func(s *quoteSpec) { s.chain = w.chain(leaves["up-to-date"], false) }), good, "", goodResult)
	caPem, rootPem := pemCert(w.ca.der), pemCert(w.root.der)
	add("anchors-ca-and-root", up, good, "", goodResult).Anchors = hex.EncodeToString(append(caPem, rootPem...))
	add("anchor-ca-without-root", up, good, "", "error:pck-chain-mismatch").Anchors = hex.EncodeToString(caPem)
	add("anchor-ca-foreign-root", up, good, "", "error:pck-chain-mismatch").Anchors = hex.EncodeToString(append(caPem, pemCert(w.otherRoot.der)...))
	otherCa := issue(certSpec{serial: 2, name: caName, key: "collateral other pck ca", ca: true, pathLen: 0, notBefore: date(2020, 1, 1), notAfter: date(2040, 1, 1)}, w.root)
	offPath := quote(leaves["up-to-date"], 8, func(s *quoteSpec) {
		s.chain = append(append(pemCert(leaves["up-to-date"].der), pemCert(otherCa.der)...), rootPem...)
	})
	add("chain-ca-off-path", offPath, good, "", "error:pck-chain-mismatch").Anchors = hex.EncodeToString(append(caPem, rootPem...))
	return file
}

func bigInt(v int64) *big.Int { return big.NewInt(v) }

const goodResult = "status=good platform=up-to-date qe=up-to-date date=2025-02-12T00:00:00Z advisories="

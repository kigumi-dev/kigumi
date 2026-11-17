package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

var qvlStatuses = []string{"UpToDate", "SWHardeningNeeded", "ConfigurationNeeded", "ConfigurationAndSWHardeningNeeded", "OutOfDate", "OutOfDateConfigurationNeeded", "Revoked"}

var statusNames = map[string]string{
	"UpToDate": "good", "SWHardeningNeeded": "sw-hardening-needed", "ConfigurationNeeded": "configuration-needed",
	"ConfigurationAndSWHardeningNeeded": "configuration-and-sw-hardening-needed", "OutOfDate": "out-of-date",
	"OutOfDateConfigurationNeeded": "out-of-date-configuration-needed", "Revoked": "revoked",
}

// qvlConverge ports Intel QVL's convergeTcbStatuses with its
// VALID_QE_STATUSES table (EvaluateTcb.cpp).
func qvlConverge(platform, qe string) string {
	switch qe {
	case "SWHardeningNeeded", "ConfigurationAndSWHardeningNeeded":
		return "error:tcb-not-supported"
	case "Revoked":
		return "revoked"
	case "OutOfDate":
		switch platform {
		case "UpToDate", "SWHardeningNeeded":
			return "out-of-date"
		case "ConfigurationNeeded", "ConfigurationAndSWHardeningNeeded":
			return "out-of-date-configuration-needed"
		}
	}
	return statusNames[platform]
}

func replaceOnce(old, new string) func([]byte) []byte {
	return func(raw []byte) []byte {
		if !bytes.Contains(raw, []byte(old)) {
			panic("fixture edit does not apply: " + old)
		}
		return bytes.Replace(raw, []byte(old), []byte(new), 1)
	}
}

func tcbV2Document(levels []level) []byte {
	var b strings.Builder
	b.WriteString(`{"tcbInfo":{"version":2,"issueDate":"2025-05-25T00:00:00Z","nextUpdate":"2025-06-24T00:00:00Z","fmspc":"00906ED50000","pceId":"0000","tcbType":0,"tcbEvaluationDataNumber":17,"tcbLevels":[`)
	for i, l := range levels {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"tcb":{`)
		for j, svn := range l.components {
			fmt.Fprintf(&b, `"sgxtcbcomp%02dsvn":%d,`, j+1, svn)
		}
		fmt.Fprintf(&b, `"pcesvn":%d},"tcbDate":"%s","tcbStatus":"%s"}`, l.pceSvn, l.date.Format(pcsTime), l.status)
	}
	b.WriteString(`]},"signature":"` + strings.Repeat("00", 64) + `"}`)
	return []byte(b.String())
}

// partsSuite checks each collateral component on its own, without a quote.
func partsSuite(w *world) suite {
	file := suite{Primitive: "sgx-collateral", Blobs: blobs{},
		Source: "Collateral components against the same synthetic hierarchy: QVL TcbInfo / EnclaveIdentity parsing and level selection, convergeTcbStatuses, TCB signing chain and CRL checks, PCK SGX extension"}
	b := blobs(file.Blobs)
	levels := fixtureLevels()
	body := tcbInfoBody(fixtureFmsp, fixturePce, issuedAt, nextUpdate, 17, levels)
	plain := signedDocument("tcbInfo", body, w.signer.key, nil)
	b.put("tcb", plain)
	b.put("tcb-v2", tcbV2Document(levels))
	b.put("tcb-pretty", bytes.ReplaceAll(bytes.ReplaceAll(plain, []byte(`,"`), []byte(",\n  \"")), []byte(`:{`), []byte(": {")))
	b.put("tcb-reordered", signedDocument("tcbInfo", body, w.signer.key, replaceOnce(`"fmspc":"00906ED50000","pceId":"0000"`, `"pceId":"0000","fmspc":"00906ED50000"`)))
	flipped := append([]byte(nil), plain...)
	at := bytes.Index(flipped, []byte(`"signature":"`)) + len(`"signature":"`)
	flipped[at] = "01"[map[bool]int{true: 1}[flipped[at] == '0']]
	b.put("tcb-flipped", flipped)
	b.put("tcb-expired", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, date(2025, 4, 1), date(2025, 5, 1), 16, levels), w.signer.key, nil))
	b.put("tcb-future", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, date(2025, 6, 2), date(2025, 7, 2), 18, levels), w.signer.key, nil))
	b.put("chain", w.signingChain(w.signer, w.root))
	b.put("chain-short", pemCert(w.signer.der))
	b.put("root-crl", w.rootCrl())
	b.put("root-crl-revokes-signer", w.rootCrl(w.signer))
	b.put("root-crl-expired", crl(w.root, crlSpec{thisUpdate: date(2025, 4, 1), nextUpdate: date(2025, 5, 1)}))
	b.put("root-crl-critical", crl(w.root, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, critical: true}))
	b.put("root-crl-unnumbered", crl(w.root, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, noNumber: true}))
	misnamed := issue(certSpec{serial: 4, name: "Kigumi Fixture Collateral Signer", key: "collateral tcb signer", notBefore: date(2024, 1, 1), notAfter: date(2030, 1, 1)}, w.root)
	b.put("chain-misnamed", w.signingChain(misnamed, w.root))
	short := issue(certSpec{serial: 5, name: signerName, key: "collateral short signer", notBefore: date(2024, 1, 1), notAfter: date(2025, 5, 31)}, w.root)
	b.put("chain-short-lived", w.signingChain(short, w.root))
	b.put("tcb-short-lived", signedDocument("tcbInfo", body, short.key, nil))
	late := issue(certSpec{serial: 6, name: signerName, key: "collateral late signer", notBefore: verifyAt.Add(24 * time.Hour), notAfter: date(2030, 1, 1)}, w.root)
	b.put("chain-not-yet-valid", w.signingChain(late, w.root))
	b.put("tcb-not-yet-valid-signer", signedDocument("tcbInfo", body, late.key, nil))
	b.put("qe", signedDocument("enclaveIdentity", qeIdentityBody("QE", issuedAt, nextUpdate, 17, fixtureQeLevels()), w.signer.key, nil))
	b.put("qe-tampered", signedDocument("enclaveIdentity", qeIdentityBody("QE", issuedAt, nextUpdate, 17, fixtureQeLevels()), w.signer.key, replaceOnce(`"isvprodid":1`, `"isvprodid":2`)))
	little := qeIdentityBody("QE", issuedAt, nextUpdate, 17, fixtureQeLevels())
	little.Miscselect = "01000000"
	b.put("qe-misc-le", signedDocument("enclaveIdentity", little, w.signer.key, nil))
	unmasked := qeIdentityBody("QE", issuedAt, nextUpdate, 17, fixtureQeLevels())
	unmasked.Attributes, unmasked.AttributesMask = "15000000000000000000000000000000", "FBFFFFFFFFFFFFFF0000000000000000"
	b.put("qe-unmasked", signedDocument("enclaveIdentity", unmasked, w.signer.key, nil))
	b.put("qe-qve", signedDocument("enclaveIdentity", qeIdentityBody("QVE", issuedAt, nextUpdate, 17, fixtureQeLevels()), w.signer.key, nil))
	v1 := qeIdentityBody("QE", issuedAt, nextUpdate, 17, fixtureQeLevels())
	v1.Version = 1
	b.put("qe-v1", signedDocument("enclaveIdentity", v1, w.signer.key, nil))
	duplicate := qeIdentityBody("QE", issuedAt, nextUpdate, 17, append(fixtureQeLevels(), fixtureQeLevels()[0]))
	b.put("qe-duplicate", signedDocument("enclaveIdentity", duplicate, w.signer.key, nil))
	for name, edit := range map[string]func([]byte) []byte{
		"tcb-escape":     replaceOnce(`"tcbStatus":"UpToDate"`, `"tcbStatus":"UpTo\u0044ate"`),
		"tcb-float":      replaceOnce(`"tcbType":0`, `"tcbType":0.0`),
		"tcb-duplicate":  replaceOnce(`"tcbType":0`, `"tcbType":0,"tcbType":0`),
		"tcb-tdx":        replaceOnce(`"id":"SGX"`, `"id":"TDX"`),
		"tcb-tdx-module": replaceOnce(`"tcbType":0`, `"tdxModule":{},"tcbType":0`),
		"tcb-type":       replaceOnce(`"tcbType":0`, `"tcbType":1`),
		"tcb-version":    replaceOnce(`"version":3`, `"version":4`),
		"tcb-date":       replaceOnce(`"tcbDate":"2025-02-12T00:00:00Z"`, `"tcbDate":"2025-02-30T00:00:00Z"`),
		"tcb-zone":       replaceOnce(`"issueDate":"2025-05-25T00:00:00Z"`, `"issueDate":"2025-05-25T00:00:00+00:00"`),
		"tcb-status":     replaceOnce(`"tcbStatus":"Revoked"`, `"tcbStatus":"TcbRevoked"`),
		"tcb-levels":     replaceOnce(`{"tcb":{"sgxtcbcomponents":[{"svn":1,"category":"BIOS"},`, `{"tcb":{"sgxtcbcomponents":[`),
		"tcb-mixed":      replaceOnce(`{"tcb":{"sgxtcbcomponents":[{"svn":1,"category":"BIOS"},{"svn":1},{"svn":1},{"svn":1},{"svn":1},{"svn":1},{"svn":0},{"svn":1},{"svn":0},{"svn":0},{"svn":0},{"svn":0},{"svn":0},{"svn":0},{"svn":0},{"svn":0}],"pcesvn":9}`, `{"tcb":{"sgxtcbcomp01svn":1,"sgxtcbcomp02svn":1,"sgxtcbcomp03svn":1,"sgxtcbcomp04svn":1,"sgxtcbcomp05svn":1,"sgxtcbcomp06svn":1,"sgxtcbcomp07svn":0,"sgxtcbcomp08svn":1,"sgxtcbcomp09svn":0,"sgxtcbcomp10svn":0,"sgxtcbcomp11svn":0,"sgxtcbcomp12svn":0,"sgxtcbcomp13svn":0,"sgxtcbcomp14svn":0,"sgxtcbcomp15svn":0,"sgxtcbcomp16svn":0,"pcesvn":9}`),
		"tcb-trailing":   func(raw []byte) []byte { return raw },
	} {
		doc := signedDocument("tcbInfo", body, w.signer.key, edit)
		if name == "tcb-trailing" {
			doc = append(doc, []byte(" {}")...)
		}
		b.put(name, doc)
	}
	b.put("tcb-unsigned", bytes.Replace(plain, []byte(`,"signature":"`+string(plain[len(plain)-130:len(plain)-2])+`"`), nil, 1))
	b.put("tcb-duplicate-level", signedDocument("tcbInfo", tcbInfoBody(fixtureFmsp, fixturePce, issuedAt, nextUpdate, 17, append(levels, levels[0])), w.signer.key, nil))
	root := hex.EncodeToString(pemCert(w.root.der))
	now := millis(verifyAt)
	add := func(v vector) *vector {
		file.Vectors = append(file.Vectors, v)
		return &file.Vectors[len(file.Vectors)-1]
	}
	tcb := func(id, doc, name string, expected string) {
		p := platformFor[name]
		add(vector{ID: id, Op: "sgx-tcb", Collateral: &collateralRef{TcbInfo: doc}, CpuSvn: hex.EncodeToString(p.components[:]), PceSvn: uint16(p.pceSvn), Expected: expected})
	}
	tcb("level-up-to-date", "tcb", "up-to-date", "ok status=up-to-date date=2025-02-12T00:00:00Z advisories=")
	tcb("level-sw-hardening", "tcb", "sw-hardening", "ok status=sw-hardening-needed date=2025-02-12T00:00:00Z advisories=INTEL-SA-00334")
	tcb("level-configuration", "tcb", "configuration", "ok status=configuration-needed date=2025-02-12T00:00:00Z advisories=INTEL-SA-00161")
	tcb("level-configuration-and-sw", "tcb", "configuration-and-sw", "ok status=configuration-and-sw-hardening-needed date=2025-02-12T00:00:00Z advisories=INTEL-SA-00161,INTEL-SA-00233")
	tcb("level-out-of-date", "tcb", "out-of-date", "ok status=out-of-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615")
	tcb("level-out-of-date-configuration", "tcb", "out-of-date-configuration", "ok status=out-of-date-configuration-needed date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615,INTEL-SA-00657")
	tcb("level-revoked", "tcb", "revoked", "ok status=revoked date=2022-03-09T00:00:00Z advisories=INTEL-SA-00219")
	tcb("level-not-supported", "tcb", "not-supported", "error:tcb-not-supported")
	tcb("level-high-components-low-pce", "tcb", "high-components-low-pce", "ok status=out-of-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615")
	tcb("level-one-component-below", "tcb", "one-component-below", "ok status=out-of-date date=2024-08-14T00:00:00Z advisories=INTEL-SA-00615")
	tcb("v2-layout", "tcb-v2", "configuration", "ok status=configuration-needed date=2025-02-12T00:00:00Z advisories=")
	tcb("pretty-printed", "tcb-pretty", "up-to-date", "ok status=up-to-date date=2025-02-12T00:00:00Z advisories=")
	for _, c := range []struct{ doc, expected string }{
		{"tcb-escape", "error:bad-tcb-info"}, {"tcb-float", "error:bad-tcb-info"}, {"tcb-duplicate", "error:bad-tcb-info"},
		{"tcb-tdx", "error:tcb-info-mismatch"}, {"tcb-tdx-module", "error:bad-tcb-info"}, {"tcb-type", "error:bad-tcb-info"},
		{"tcb-version", "error:bad-tcb-info"}, {"tcb-date", "error:bad-tcb-info"}, {"tcb-zone", "error:bad-tcb-info"},
		{"tcb-status", "error:bad-tcb-info"}, {"tcb-levels", "error:bad-tcb-info"}, {"tcb-mixed", "error:bad-tcb-info"},
		{"tcb-trailing", "error:bad-tcb-info"}, {"tcb-unsigned", "error:bad-tcb-info"}, {"tcb-duplicate-level", "error:bad-tcb-info"},
	} {
		tcb("parse-"+strings.TrimPrefix(c.doc, "tcb-"), c.doc, "up-to-date", c.expected)
	}

	qeReport := func(isvSvn uint16, edit func(*reportBody)) string {
		r := reportBody{cpuSvn: make([]byte, 16), attributes: decode("15000000000000000700000000000000"), mrEnclave: bytes.Repeat([]byte{0x55}, 32),
			mrSigner: qeMrSigner, isvProdID: 1, isvSvn: isvSvn, reportData: make([]byte, 64)}
		if edit != nil {
			edit(&r)
		}
		return hex.EncodeToString(r.bytes())
	}
	qe := func(id, doc, report, expected string) {
		add(vector{ID: id, Op: "sgx-qe-identity", Collateral: &collateralRef{QeIdentity: doc}, Hex: report, Expected: expected})
	}
	qe("qe-up-to-date", "qe", qeReport(8, nil), "ok status=up-to-date isvSvn=8 date=2025-02-12T00:00:00Z advisories=")
	qe("qe-newer", "qe", qeReport(11, nil), "ok status=up-to-date isvSvn=11 date=2025-02-12T00:00:00Z advisories=")
	qe("qe-out-of-date", "qe", qeReport(7, nil), "ok status=out-of-date isvSvn=7 date=2024-03-13T00:00:00Z advisories=INTEL-SA-00477")
	qe("qe-revoked", "qe", qeReport(3, nil), "ok status=revoked isvSvn=3 date=2023-02-15T00:00:00Z advisories=INTEL-SA-00202")
	qe("qe-not-supported", "qe", qeReport(2, nil), "error:tcb-not-supported")
	qe("qe-masked-attribute", "qe", qeReport(8, func(r *reportBody) { r.attributes = decode("11000000000000000300000000000000") }), "ok status=up-to-date isvSvn=8 date=2025-02-12T00:00:00Z advisories=")
	qe("qe-attribute-mismatch", "qe", qeReport(8, func(r *reportBody) { r.attributes = decode("13000000000000000700000000000000") }), "error:qe-identity-mismatch")
	qe("qe-misc-mismatch", "qe", qeReport(8, func(r *reportBody) { r.misc = 1 }), "error:qe-identity-mismatch")
	qe("qe-mrsigner-mismatch", "qe", qeReport(8, func(r *reportBody) { r.mrSigner = bytes.Repeat([]byte{0x44}, 32) }), "error:qe-identity-mismatch")
	qe("qe-prodid-mismatch", "qe", qeReport(8, func(r *reportBody) { r.isvProdID = 2 }), "error:qe-identity-mismatch")
	qe("qe-misc-little-endian", "qe-misc-le", qeReport(8, func(r *reportBody) { r.misc = 1 }), "ok status=up-to-date isvSvn=8 date=2025-02-12T00:00:00Z advisories=")
	qe("qe-misc-required", "qe-misc-le", qeReport(8, nil), "error:qe-identity-mismatch")
	qe("qe-expected-not-masked", "qe-unmasked", qeReport(8, nil), "error:qe-identity-mismatch")
	qe("qe-other-enclave", "qe-qve", qeReport(8, nil), "ok status=up-to-date isvSvn=8 date=2025-02-12T00:00:00Z advisories=")
	qe("qe-version-1", "qe-v1", qeReport(8, nil), "error:bad-qe-identity")
	qe("qe-duplicate-level", "qe-duplicate", qeReport(8, nil), "error:bad-qe-identity")

	for _, platform := range qvlStatuses {
		for _, qeStatus := range qvlStatuses {
			add(vector{ID: "converge-" + platform + "-" + qeStatus, Op: "sgx-converge", Args: []string{platform, qeStatus}, Expected: strings.Replace("ok "+qvlConverge(platform, qeStatus), "ok error:", "error:", 1)})
		}
	}

	auth := func(id, kind string, c collateralRef, anchors string, at int64, expected string) {
		add(vector{ID: id, Op: "sgx-collateral-auth", Args: []string{kind}, Collateral: &c, Anchors: anchors, Now: at, Expected: expected})
	}
	tcbAuth := collateralRef{TcbInfo: "tcb", TcbChain: "chain", RootCrl: "root-crl"}
	auth("tcb-authentic", "tcb", tcbAuth, root, now, "ok")
	auth("tcb-pretty-authentic", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbInfo = "tcb-pretty" }), root, now, "ok")
	auth("tcb-next-update-boundary", "tcb", tcbAuth, root, millis(nextUpdate), "ok")
	auth("tcb-member-order", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbInfo = "tcb-reordered" }), root, now, "error:collateral-signature-invalid")
	auth("tcb-signature-flipped", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbInfo = "tcb-flipped" }), root, now, "error:collateral-signature-invalid")
	auth("tcb-expired", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbInfo = "tcb-expired" }), root, now, "error:collateral-expired")
	auth("tcb-issued-after-now", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbInfo = "tcb-future" }), root, now, "ok")
	auth("signer-not-yet-valid", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbChain = "chain-not-yet-valid"; c.TcbInfo = "tcb-not-yet-valid-signer" }), root, now, "ok")
	auth("chain-without-root", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbChain = "chain-short" }), root, now, "error:bad-signing-chain")
	auth("chain-misnamed", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbChain = "chain-misnamed" }), root, now, "error:bad-signing-chain")
	auth("signer-expired", "tcb", *tcbAuth.with(func(c *collateralRef) { c.TcbChain = "chain-short-lived"; c.TcbInfo = "tcb-short-lived" }), root, now, "error:collateral-expired")
	auth("signer-revoked", "tcb", *tcbAuth.with(func(c *collateralRef) { c.RootCrl = "root-crl-revokes-signer" }), root, now, "error:certificate-revoked")
	auth("root-crl-expired", "tcb", *tcbAuth.with(func(c *collateralRef) { c.RootCrl = "root-crl-expired" }), root, now, "error:collateral-expired")
	auth("root-crl-critical", "tcb", *tcbAuth.with(func(c *collateralRef) { c.RootCrl = "root-crl-critical" }), root, now, "error:bad-crl")
	auth("root-crl-unnumbered", "tcb", *tcbAuth.with(func(c *collateralRef) { c.RootCrl = "root-crl-unnumbered" }), root, now, "error:bad-crl")
	auth("other-root-pinned", "tcb", tcbAuth, hex.EncodeToString(pemCert(w.otherRoot.der)), now, "error:bad-crl")
	qeAuth := collateralRef{QeIdentity: "qe", QeChain: "chain", RootCrl: "root-crl"}
	auth("qe-authentic", "qe", qeAuth, root, now, "ok")
	auth("qe-tampered", "qe", *qeAuth.with(func(c *collateralRef) { c.QeIdentity = "qe-tampered" }), root, now, "error:collateral-signature-invalid")
	auth("qe-after-next-update", "qe", qeAuth, root, millis(nextUpdate)+1, "error:collateral-expired")

	leaf := w.pck(platformFor["up-to-date"], sgxExtensionEdit{}, true)
	other := w.pck(platformFor["up-to-date"], sgxExtensionEdit{}, true)
	revoking := w.pckCrl(leaf)
	crlCase := func(id string, list []byte, issuer, cert *authority, expected string) {
		add(vector{ID: id, Op: "sgx-crl", Hex: hex.EncodeToString(list), Anchors: hex.EncodeToString(pemCert(issuer.der)), Hex2: hex.EncodeToString(pemCert(cert.der)), Expected: expected})
	}
	crlCase("crl-revokes-leaf", revoking, w.ca, leaf, "ok version=2 revoked=1 number=01 unsupported=0 revokes=true")
	crlCase("crl-spares-other", revoking, w.ca, other, "ok version=2 revoked=1 number=01 unsupported=0 revokes=false")
	crlCase("crl-root", w.rootCrl(), w.root, w.ca, "ok version=2 revoked=0 number=01 unsupported=0 revokes=false")
	crlCase("crl-critical", crl(w.root, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, critical: true}), w.root, w.ca, "ok version=2 revoked=0 number=01 unsupported=1 revokes=false")
	crlCase("crl-unnumbered", crl(w.root, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, noNumber: true}), w.root, w.ca, "ok version=2 revoked=0 number=none unsupported=0 revokes=false")
	crlCase("crl-wrong-issuer", revoking, w.root, leaf, "error:verify")
	crlCase("crl-foreign-key", crl(w.ca, crlSpec{thisUpdate: crlThis, nextUpdate: crlNext, signer: w.root}), w.ca, leaf, "error:verify")
	tampered := append([]byte(nil), revoking...)
	tampered[len(tampered)-1] ^= 1
	crlCase("crl-tampered", tampered, w.ca, leaf, "error:verify")
	crlCase("crl-trailing", append(append([]byte(nil), revoking...), 0), w.ca, leaf, "error:parse")

	ext := func(id string, cert *authority, expected string) {
		add(vector{ID: id, Op: "sgx-pck-extension", Hex: hex.EncodeToString(pemCert(cert.der)), Expected: expected})
	}
	ext("extension", leaf, "ok fmspc=00906ed50000 pceId=0000 components=04040303040100030000000000000000 pceSvn=13 cpuSvn=04040303040100030000000000000000 type=0")
	ext("extension-seven-members", w.pck(platformFor["configuration"], sgxExtensionEdit{platform: true}, true), "ok fmspc=00906ed50000 pceId=0000 components=03030303040100030000000000000000 pceSvn=13 cpuSvn=03030303040100030000000000000000 type=0")
	ext("extension-missing", w.pck(platformFor["up-to-date"], sgxExtensionEdit{}, false), "error:bad-pck-extension")
	ext("extension-no-fmspc", w.pck(platformFor["up-to-date"], sgxExtensionEdit{dropFmspc: true}, true), "error:bad-pck-extension")
	ext("extension-wide-svn", w.pck(platformFor["up-to-date"], sgxExtensionEdit{wideSvn: true}, true), "error:bad-pck-extension")
	ext("extension-repeated", w.pck(platformFor["up-to-date"], sgxExtensionEdit{repeated: true}, true), "error:bad-pck-extension")
	ext("extension-on-ca", w.ca, "error:bad-pck-extension")
	return file
}

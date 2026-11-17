package driver_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"strings"
	"testing"
)

func collateralPartOracle(t *testing.T, blobs map[string]string, v sgxVector) {
	t.Helper()
	switch v.Op {
	case "sgx-converge":
		want := strings.TrimPrefix(v.Expected, "ok ")
		if got := oracleConverge(v.Args[0], v.Args[1]); got != want {
			t.Fatalf("QVL convergence gives %s; fixture says %s", got, v.Expected)
		}
	case "sgx-tcb":
		if strings.HasPrefix(v.Expected, "error:") && v.Expected != "error:tcb-not-supported" {
			return
		}
		body, _ := oracleSigned(t, oracleBlob(t, blobs, v.Collateral.TcbInfo), "tcbInfo")
		_, levels := oracleLevels(t, body)
		platform := []int{}
		for _, b := range mustHex(t, v.CpuSvn) {
			platform = append(platform, int(b))
		}
		level := oracleSelect(levels, append(platform, int(v.PceSvn)))
		if level == nil {
			if v.Expected != "error:tcb-not-supported" {
				t.Fatalf("no level matches; fixture says %s", v.Expected)
			}
			return
		}
		want := oracleFields(v.Expected)
		if want["status"] != oracleTcbNames[level.status] || want["date"] != level.date || want["advisories"] != strings.Join(level.advisories, ",") {
			t.Fatalf("QVL reading selects %s %s %v; fixture says %s", level.status, level.date, level.advisories, v.Expected)
		}
	case "sgx-crl":
		crlOracle(t, v)
	case "sgx-collateral-auth":
		authOracle(t, blobs, v)
	case "sgx-root":
		cert := sgxPemCertificates(t, mustHex(t, v.Hex))[0]
		key, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok || key.Curve != elliptic.P256() || !cert.IsCA || cert.MaxPathLen != 1 || cert.CheckSignatureFrom(cert) != nil {
			t.Fatalf("root is not a self-signed P-256 CA with pathLen 1")
		}
	}
}

func crlOracle(t *testing.T, v sgxVector) {
	t.Helper()
	if v.Expected == "error:parse" {
		var outer asn1.RawValue
		if rest, err := asn1.Unmarshal(mustHex(t, v.Hex), &outer); err == nil && len(rest) == 0 {
			t.Fatal("the CRL is one complete DER element, yet the fixture expects a parse error")
		}
		return
	}
	list, err := x509.ParseRevocationList(mustHex(t, v.Hex))
	if err != nil {
		if v.Expected != "error:parse" {
			t.Fatalf("Go refuses the CRL (%v); fixture says %s", err, v.Expected)
		}
		return
	}
	issuer := sgxPemCertificates(t, mustHex(t, v.Anchors))[0]
	signed := list.CheckSignatureFrom(issuer) == nil
	if signed != (v.Expected != "error:verify") {
		t.Fatalf("Go signature check = %v; fixture says %s", signed, v.Expected)
	}
	if !signed {
		return
	}
	cert := sgxPemCertificates(t, mustHex(t, v.Hex2))[0]
	revokes := false
	for _, entry := range list.RevokedCertificateEntries {
		revokes = revokes || entry.SerialNumber.Cmp(cert.SerialNumber) == 0
	}
	want := oracleFields(v.Expected)
	if want["revokes"] != map[bool]string{true: "true", false: "false"}[revokes] {
		t.Fatalf("Go says revokes=%v; fixture says %s", revokes, v.Expected)
	}
}

func authOracle(t *testing.T, blobs map[string]string, v sgxVector) {
	t.Helper()
	if v.Expected != "ok" && v.Expected != "error:collateral-signature-invalid" {
		return
	}
	doc, chain, key := v.Collateral.TcbInfo, v.Collateral.TcbChain, "tcbInfo"
	if v.Args[0] == "qe" {
		doc, chain, key = v.Collateral.QeIdentity, v.Collateral.QeChain, "enclaveIdentity"
	}
	signer := sgxPemCertificates(t, oracleBlob(t, blobs, chain))[0]
	body, signature := oracleSigned(t, oracleBlob(t, blobs, doc), key)
	valid := oracleSignatureValid(signer.PublicKey.(*ecdsa.PublicKey), body, signature)
	if valid != (v.Expected == "ok") {
		t.Fatalf("Go signature check = %v over %s; fixture says %s", valid, hex.EncodeToString(body[:16]), v.Expected)
	}
	if v.Expected == "ok" {
		root := sgxPemCertificates(t, mustHex(t, v.Anchors))[0]
		if signer.CheckSignatureFrom(root) != nil {
			t.Fatal("signing certificate is not issued by the pinned root")
		}
		oracleCrlClean(t, oracleBlob(t, blobs, v.Collateral.RootCrl), root, signer)
	}
}

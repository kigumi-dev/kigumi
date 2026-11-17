package driver_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strings"
	"testing"
)

// An independent Go reading of Intel QVL's collateral rules for the fixture expectations.

type oracleLevel struct {
	key        []int
	status     string
	date       string
	advisories []string
}

var oracleTcbNames = map[string]string{
	"UpToDate": "up-to-date", "SWHardeningNeeded": "sw-hardening-needed", "ConfigurationNeeded": "configuration-needed",
	"ConfigurationAndSWHardeningNeeded": "configuration-and-sw-hardening-needed", "OutOfDate": "out-of-date",
	"OutOfDateConfigurationNeeded": "out-of-date-configuration-needed", "Revoked": "revoked",
}

func oracleConverge(platform, qe string) string {
	if qe == "SWHardeningNeeded" || qe == "ConfigurationAndSWHardeningNeeded" {
		return "error:tcb-not-supported"
	}
	if qe == "Revoked" {
		return "revoked"
	}
	if qe == "OutOfDate" && (platform == "UpToDate" || platform == "SWHardeningNeeded") {
		return "out-of-date"
	}
	if qe == "OutOfDate" && strings.HasPrefix(platform, "Configuration") {
		return "out-of-date-configuration-needed"
	}
	if platform == "UpToDate" {
		return "good"
	}
	return oracleTcbNames[platform]
}

func oracleSigned(t *testing.T, doc []byte, key string) (body, signature []byte) {
	t.Helper()
	var top map[string]json.RawMessage
	if err := json.Unmarshal(doc, &top); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, top[key]); err != nil {
		t.Fatal(err)
	}
	var sigHex string
	if err := json.Unmarshal(top["signature"], &sigHex); err != nil {
		t.Fatal(err)
	}
	signature, err := hex.DecodeString(sigHex)
	if err != nil {
		t.Fatal(err)
	}
	return compact.Bytes(), signature
}

func oracleSignatureValid(key *ecdsa.PublicKey, body, signature []byte) bool {
	digest := sha256.Sum256(body)
	return len(signature) == 64 && ecdsa.Verify(key, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:]))
}

// oracleLevels returns the TCB Info or QE Identity levels in QVL's order.
func oracleLevels(t *testing.T, body []byte) (map[string]any, []oracleLevel) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	var levels []oracleLevel
	for _, raw := range doc["tcbLevels"].([]any) {
		entry := raw.(map[string]any)
		tcb := entry["tcb"].(map[string]any)
		var key []int
		if isvsvn, ok := tcb["isvsvn"]; ok {
			key = []int{int(isvsvn.(float64))}
		} else if components, ok := tcb["sgxtcbcomponents"]; ok {
			for _, c := range components.([]any) {
				key = append(key, int(c.(map[string]any)["svn"].(float64)))
			}
		} else {
			for i := 1; i <= 16; i++ {
				key = append(key, int(tcb[fmt.Sprintf("sgxtcbcomp%02dsvn", i)].(float64)))
			}
		}
		if pce, ok := tcb["pcesvn"]; ok {
			key = append(key, int(pce.(float64)))
		}
		level := oracleLevel{key: key, status: entry["tcbStatus"].(string), date: entry["tcbDate"].(string)}
		if ids, ok := entry["advisoryIDs"].([]any); ok {
			for _, id := range ids {
				level.advisories = append(level.advisories, id.(string))
			}
		}
		levels = append(levels, level)
	}
	sort.SliceStable(levels, func(i, j int) bool { return slices.Compare(levels[i].key, levels[j].key) > 0 })
	return doc, levels
}

func oracleSelect(levels []oracleLevel, platform []int) *oracleLevel {
	for i := range levels {
		reached := true
		for j, v := range levels[i].key {
			reached = reached && platform[j] >= v
		}
		if reached {
			return &levels[i]
		}
	}
	return nil
}

type oraclePck struct {
	platform []int
	fmspc    string
}

func oraclePckExtension(t *testing.T, cert *x509.Certificate) *oraclePck {
	t.Helper()
	type member struct {
		ID    asn1.ObjectIdentifier
		Value asn1.RawValue
	}
	for _, ext := range cert.Extensions {
		if ext.Id.String() != "1.2.840.113741.1.13.1" {
			continue
		}
		var members []member
		if _, err := asn1.Unmarshal(ext.Value, &members); err != nil {
			t.Fatal(err)
		}
		out := &oraclePck{}
		for _, m := range members {
			switch m.ID.String() {
			case "1.2.840.113741.1.13.1.2":
				var tcb []member
				if _, err := asn1.Unmarshal(m.Value.FullBytes, &tcb); err != nil {
					t.Fatal(err)
				}
				for _, c := range tcb[:17] {
					var v int
					if _, err := asn1.Unmarshal(c.Value.FullBytes, &v); err != nil {
						t.Fatal(err)
					}
					out.platform = append(out.platform, v)
				}
			case "1.2.840.113741.1.13.1.4":
				out.fmspc = hex.EncodeToString(m.Value.Bytes)
			}
		}
		return out
	}
	return nil
}

func oracleBlob(t *testing.T, blobs map[string]string, name string) []byte {
	t.Helper()
	data, err := hex.DecodeString(blobs[name])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func oracleFields(expected string) map[string]string {
	fields := map[string]string{}
	for _, part := range strings.Fields(expected) {
		if key, value, ok := strings.Cut(part, "="); ok {
			fields[key] = value
		}
	}
	return fields
}

func oracleCrlClean(t *testing.T, der []byte, issuer *x509.Certificate, certs ...*x509.Certificate) {
	t.Helper()
	list, err := x509.ParseRevocationList(der)
	if err != nil || list.CheckSignatureFrom(issuer) != nil {
		t.Fatalf("CRL from %s does not verify: %v", issuer.Subject, err)
	}
	for _, cert := range certs {
		for _, entry := range list.RevokedCertificateEntries {
			if entry.SerialNumber.Cmp(cert.SerialNumber) == 0 {
				t.Fatalf("%s is revoked in a positive vector", cert.Subject)
			}
		}
	}
}

func verifyCollateralOracle(t *testing.T, blobs map[string]string, v sgxVector, quote []byte) {
	t.Helper()
	sig := quote[436:]
	authLen := int(binary.LittleEndian.Uint16(sig[576:578]))
	chain := sgxPemCertificates(t, sig[578+authLen+6:])
	root := oracleSelfSignedAnchor(t, v.Anchors)
	signers := sgxPemCertificates(t, oracleBlob(t, blobs, v.Collateral.TcbChain))
	if signers[0].CheckSignatureFrom(root) != nil || chain[1].CheckSignatureFrom(root) != nil {
		t.Fatal("signing or PCK CA certificate is not issued by the pinned root")
	}
	oracleCrlClean(t, oracleBlob(t, blobs, v.Collateral.RootCrl), root, chain[1], signers[0])
	oracleCrlClean(t, oracleBlob(t, blobs, v.Collateral.PckCrl), chain[1], chain[0])
	key := signers[0].PublicKey.(*ecdsa.PublicKey)
	tcbBody, tcbSig := oracleSigned(t, oracleBlob(t, blobs, v.Collateral.TcbInfo), "tcbInfo")
	qeBody, qeSig := oracleSigned(t, oracleBlob(t, blobs, v.Collateral.QeIdentity), "enclaveIdentity")
	if !oracleSignatureValid(key, tcbBody, tcbSig) || !oracleSignatureValid(key, qeBody, qeSig) {
		t.Fatal("collateral signature does not verify")
	}
	pck := oraclePckExtension(t, chain[0])
	tcbDoc, tcbLevels := oracleLevels(t, tcbBody)
	if !strings.EqualFold(tcbDoc["fmspc"].(string), pck.fmspc) {
		t.Fatal("TCB Info FMSPC is not the PCK certificate's")
	}
	_, qeLevels := oracleLevels(t, qeBody)
	platform := oracleSelect(tcbLevels, pck.platform)
	qe := oracleSelect(qeLevels, []int{int(binary.LittleEndian.Uint16(sig[128+0x102:]))})
	if platform == nil || qe == nil {
		t.Fatal("no TCB level matches a positive vector")
	}
	want := oracleFields(v.Expected)
	status := oracleConverge(platform.status, qe.status)
	if want["status"] != status || want["platform"] != oracleTcbNames[platform.status] || want["qe"] != oracleTcbNames[qe.status] {
		t.Fatalf("QVL reading gives status=%s platform=%s qe=%s; fixture says %s", status, platform.status, qe.status, v.Expected)
	}
	if want["advisories"] != strings.Join(append(slices.Clone(platform.advisories), qe.advisories...), ",") {
		t.Fatalf("advisories %v %v; fixture says %s", platform.advisories, qe.advisories, v.Expected)
	}
	if v.Accept == "" && status != "good" {
		t.Fatal("the production default accepts only good")
	}
}

func mustHex(t *testing.T, text string) []byte {
	t.Helper()
	data, err := hex.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

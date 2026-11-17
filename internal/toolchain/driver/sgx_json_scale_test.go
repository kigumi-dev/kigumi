package driver_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

const sgxJsonKeys = 20000

type sgxScaleIssuer struct {
	rootPem, chainPem, crl []byte
	signer                 *ecdsa.PrivateKey
	now                    time.Time
}

func sgxJsonIssuer(t *testing.T) sgxScaleIssuer {
	t.Helper()
	start := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	signerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Kigumi JSON Scale Root"},
		NotBefore: start.AddDate(-1, 0, 0), NotAfter: start.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, SubjectKeyId: []byte{5, 6, 7, 8},
	}
	rootDer, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := x509.ParseCertificate(rootDer)
	signerTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Kigumi JSON Scale SGX TCB Signing"},
		NotBefore: rootTemplate.NotBefore, NotAfter: rootTemplate.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature}
	signerDer, err := x509.CreateCertificate(rand.Reader, signerTemplate, root, &signerKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	crl, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{Number: big.NewInt(1), ThisUpdate: start, NextUpdate: start.AddDate(0, 1, 0)}, root, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	rootPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDer})
	chain := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: signerDer}), rootPem...)
	return sgxScaleIssuer{rootPem: rootPem, chainPem: chain, crl: crl, signer: signerKey, now: start.Add(time.Hour)}
}

// sgxJsonBody is a valid SGX TCB Info body padded with `keys` unknown members.
func sgxJsonBody(keys int, duplicate bool) string {
	var b strings.Builder
	b.WriteString(`{"id":"SGX","version":3,"issueDate":"2025-05-29T00:00:00Z","nextUpdate":"2025-06-28T00:00:00Z","fmspc":"00906ED50000","pceId":"0000","tcbType":0,"tcbEvaluationDataNumber":17,"tcbLevels":[{"tcb":{"sgxtcbcomponents":[`)
	for i := 0; i < 16; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"svn":1}`)
	}
	b.WriteString(`],"pcesvn":1},"tcbDate":"2025-01-01T00:00:00Z","tcbStatus":"UpToDate"}]`)
	for i := 0; i < keys; i++ {
		fmt.Fprintf(&b, `,"k%d":0`, i)
	}
	if duplicate {
		b.WriteString(`,"k1":0`)
	}
	b.WriteString("}")
	return b.String()
}

func sgxJsonDocument(t *testing.T, body string, key *ecdsa.PrivateKey) string {
	t.Helper()
	signature := make([]byte, 64)
	if key != nil {
		digest := sha256.Sum256([]byte(body))
		r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
		if err != nil {
			t.Fatal(err)
		}
		r.FillBytes(signature[:32])
		s.FillBytes(signature[32:])
	}
	return `{"tcbInfo":` + body + `,"signature":"` + hex.EncodeToString(signature) + `"}`
}

const sgxJsonProgram = `import sgx from std/tee/sgx
import hex from std/encoding/hex
import time from std/time

fn b(text: String) -> Bytes {
    hex.decode(text).expect("hex")
}

fn s(text: String) -> String {
    String.fromBytes(b(text)).expect("text")
}

fn code(result: Result[sgx.TcbInfo, sgx.Error]) -> String {
    match result {
        Ok(info) -> "ok levels=${info.tcbLevels.len()}"
        Err(err) -> "error:${err.code.name()}"
    }
}

fn main() -> Void! {
    let clock = time.SystemClock.new()
    let chain = s(%q)
    let root = s(%q)
    let crl = b(%q)
    let unsigned = s(%q)
    let signed = s(%q)
    let duplicate = s(%q)
    let parsedAt = clock.now()
    print "parse\t${code(sgx.TcbInfo.parse(unsigned))}"
    print "ms\tparse\t${parsedAt.elapsedMillis()}"
    let unsignedAt = clock.now()
    print "unsigned\t${code(sgx.TcbInfo.parseVerified(unsigned, chain, root, crl, %d))}"
    print "ms\tunsigned\t${unsignedAt.elapsedMillis()}"
    let signedAt = clock.now()
    print "signed\t${code(sgx.TcbInfo.parseVerified(signed, chain, root, crl, %d))}"
    print "ms\tsigned\t${signedAt.elapsedMillis()}"
    print "duplicate\t${code(sgx.TcbInfo.parseVerified(duplicate, chain, root, crl, %d))}"
}
`

// TestSgxJsonScale pins that a TCB Info body with 20000 members is scanned in
// linear time, signed or not, and that a repeated member is still refused.
func TestSgxJsonScale(t *testing.T) {
	issuer := sgxJsonIssuer(t)
	at := issuer.now.UnixMilli()
	hx := func(text string) string { return hex.EncodeToString([]byte(text)) }
	body := sgxJsonBody(sgxJsonKeys, false)
	source := fmt.Sprintf(sgxJsonProgram, hex.EncodeToString(issuer.chainPem), hex.EncodeToString(issuer.rootPem), hex.EncodeToString(issuer.crl),
		hx(sgxJsonDocument(t, body, nil)), hx(sgxJsonDocument(t, body, issuer.signer)), hx(sgxJsonDocument(t, sgxJsonBody(sgxJsonKeys, true), issuer.signer)), at, at, at)
	runSgxScaleProgram(t, source, map[string]string{
		"parse":     "ok levels=1",
		"unsigned":  "error:collateral-signature-invalid",
		"signed":    "ok levels=1",
		"duplicate": "error:bad-tcb-info",
	}, map[string]int{"parse": 1000, "unsigned": 1000, "signed": 1000}, fmt.Sprintf("%d-member body", sgxJsonKeys))
}

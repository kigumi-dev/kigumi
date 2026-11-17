package driver_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os/exec"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestSgxLazyStdMatchesEager(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main/main.kg": buildSgxProgram(loadSgxCorpus(t))})
	load := func(lazy bool) *driver.Module {
		module, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg", LazyStd: lazy})
		if err != nil {
			t.Fatal(err)
		}
		return module
	}
	var previous string
	for i, lazy := range []bool{false, true} {
		res, err := driver.CheckForEditor(load(lazy))
		if err != nil {
			t.Fatalf("editor lazy=%v: %v", lazy, err)
		}
		if res.HasErrors() {
			t.Fatalf("editor lazy=%v:\n%s", lazy, res.Render())
		}
		var stderr bytes.Buffer
		ir, ok, err := driver.Compile(load(lazy), &stderr)
		if err != nil || !ok {
			t.Fatalf("compile lazy=%v: %v\n%s", lazy, err, stderr.String())
		}
		out := res.Render() + stderr.String() + ir
		if i > 0 && out != previous {
			t.Fatalf("SGX lazy/eager mismatch: lazy sha256=%x eager sha256=%x", sha256.Sum256([]byte(out)), sha256.Sum256([]byte(previous)))
		}
		previous = out
	}
}

func TestSgxFixtureGeneration(t *testing.T) {
	cmd := exec.Command("go", "run", "./testdata/conformance/sgx", "--check")
	cmd.Dir = "../../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate --check: %v\n%s", err, out)
	}
}

func TestSgxFixtureOracle(t *testing.T) {
	for _, suite := range loadSgxCorpus(t).Suites {
		for _, vector := range suite.Vectors {
			t.Run(suite.Primitive+"/"+vector.ID, func(t *testing.T) {
				input := vector.Hex
				if vector.Op == "sgx-verify" {
					input = suite.Blobs[vector.Quote]
				}
				data, err := hex.DecodeString(input)
				if err != nil {
					t.Fatal(err)
				}
				positive := !strings.HasPrefix(vector.Expected, "error:")
				switch vector.Op {
				case "sgx-pck":
					if vector.Expected != "error:unsupported-algorithm" {
						verifySgxPathOracle(t, vector, data, positive)
					}
				case "sgx-qe-binding":
					auth, err := hex.DecodeString(vector.Hex2)
					if err != nil {
						t.Fatal(err)
					}
					digest := sha256.Sum256(append(data, auth...))
					if got := hex.EncodeToString(append(digest[:], make([]byte, 32)...)); got != vector.Expected {
						t.Fatalf("binding=%s, want %s", got, vector.Expected)
					}
				case "sgx-parse", "sgx-signature", "sgx-qe", "sgx-verify":
					if positive || vector.ID == "forged-quote" || vector.ID == "forged-qe" || vector.ID == "forged" || vector.ID == "wrong-binding" {
						verifySgxQuoteOracle(t, vector, data)
					}
					if vector.Op == "sgx-verify" && positive && vector.Collateral != nil {
						verifyCollateralOracle(t, suite.Blobs, vector, data)
					}
				default:
					collateralPartOracle(t, suite.Blobs, vector)
				}
			})
		}
	}
}

func verifySgxPathOracle(t *testing.T, v sgxVector, data []byte, positive bool) {
	t.Helper()
	certs := sgxPemCertificates(t, data)
	roots := x509.NewCertPool()
	anchors, err := hex.DecodeString(v.Anchors)
	if err != nil || !roots.AppendCertsFromPEM(anchors) {
		t.Fatalf("anchors: %v", err)
	}
	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		intermediates.AddCert(cert)
	}
	_, err = certs[0].Verify(x509.VerifyOptions{
		Roots: roots, Intermediates: intermediates, CurrentTime: time.UnixMilli(v.Now),
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if (err == nil) != positive {
		t.Fatalf("Go x509 path result=%v, expected success=%v", err, positive)
	}
}

func verifySgxQuoteOracle(t *testing.T, v sgxVector, data []byte) {
	t.Helper()
	if len(data) < 1020 || int(binary.LittleEndian.Uint32(data[432:436])) != len(data)-436 {
		t.Fatal("invalid v3 signature-data envelope")
	}
	sig := data[436:]
	x, y := elliptic.Unmarshal(elliptic.P256(), append([]byte{4}, sig[64:128]...))
	if x == nil {
		t.Fatal("invalid attestation public key")
	}
	key := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	quoteValid := sgxRawSignature(&key, data[:432], sig[:64])
	wantQuote := v.ID != "forged-quote" && !(v.ID == "forged" && v.Op == "sgx-signature")
	if quoteValid != wantQuote {
		t.Fatalf("quote signature valid=%v, want %v", quoteValid, wantQuote)
	}
	if v.Op == "sgx-parse" || v.Op == "sgx-signature" {
		return
	}
	report := sig[128:512]
	authLen := int(binary.LittleEndian.Uint16(sig[576:578]))
	certAt := 578 + authLen
	if certAt+6 > len(sig) || int(binary.LittleEndian.Uint32(sig[certAt+2:certAt+6])) != len(sig)-certAt-6 {
		t.Fatal("invalid authentication/certification boundary")
	}
	digest := sha256.Sum256(append(append([]byte(nil), sig[64:128]...), sig[578:certAt]...))
	bound := bytes.Equal(report[320:352], digest[:]) && bytes.Equal(report[352:384], make([]byte, 32))
	if bound != (v.ID != "wrong-binding") {
		t.Fatalf("QE binding valid=%v", bound)
	}
	if binary.LittleEndian.Uint16(sig[certAt:]) != 5 {
		t.Fatal("positive verification fixture has no PCK chain")
	}
	certs := sgxPemCertificates(t, sig[certAt+6:])
	leaf, ok := certs[0].PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("PCK leaf is not EC")
	}
	qeValid := sgxRawSignature(leaf, report, sig[512:576])
	wantQe := v.ID != "forged-qe" && !(v.ID == "forged" && v.Op == "sgx-qe")
	if qeValid != wantQe {
		t.Fatalf("QE signature valid=%v, want %v", qeValid, wantQe)
	}
}

func sgxRawSignature(key *ecdsa.PublicKey, message, signature []byte) bool {
	digest := sha256.Sum256(message)
	return ecdsa.Verify(key, digest[:], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:]))
}

func sgxPemCertificates(t *testing.T, raw []byte) []*x509.Certificate {
	t.Helper()
	var certs []*x509.Certificate
	for block, rest := pem.Decode(raw); block != nil; block, rest = pem.Decode(rest) {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		t.Fatal("no certificates")
	}
	return certs
}

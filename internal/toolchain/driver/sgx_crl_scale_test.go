package driver_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

const sgxCrlEntries = 20001

func sgxScaleCrl(t *testing.T) (rootPem, leafPem, crl []byte, now time.Time) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2025, 5, 30, 0, 0, 0, 0, time.UTC)
	rootTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Kigumi CRL Scale Root"},
		NotBefore: start.AddDate(-1, 0, 0), NotAfter: start.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign, SubjectKeyId: []byte{1, 2, 3, 4},
	}
	rootDer, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := x509.ParseCertificate(rootDer)
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafTemplate := &x509.Certificate{SerialNumber: big.NewInt(sgxCrlEntries + 999), Subject: pkix.Name{CommonName: "Kigumi CRL Scale Leaf"},
		NotBefore: rootTemplate.NotBefore, NotAfter: rootTemplate.NotAfter}
	leafDer, err := x509.CreateCertificate(rand.Reader, leafTemplate, root, &leafKey.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]x509.RevocationListEntry, sgxCrlEntries)
	for i := range entries {
		entries[i] = x509.RevocationListEntry{SerialNumber: big.NewInt(int64(1000 + i)), RevocationTime: start}
	}
	crl, err = x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number: big.NewInt(1), ThisUpdate: start, NextUpdate: start.AddDate(0, 1, 0), RevokedCertificateEntries: entries,
	}, root, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDer}), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDer}), crl, start.Add(time.Hour)
}

const sgxScaleProgram = `import sgx from std/tee/sgx
import x509 from std/crypto/x509
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
        Ok(_) -> "ok"
        Err(err) -> "error:${err.code.name()}"
    }
}

fn detail(result: Result[sgx.TcbInfo, sgx.Error]) -> String {
    match result {
        Ok(_) -> "ok"
        Err(err) -> "error:${err.code.name()}:${err.message()}"
    }
}

// A SEQUENCE tag in the middle of the entries becomes a SET, so the list no
// longer parses and its signature no longer verifies.
fn malformed(list: Bytes) -> Bytes {
    let mut i = list.len() / 2
    for list[i] != 0x30 {
        i = i + 1
    }
    list.slice(0, i).expect("head").concat(Bytes.fill(0x31, 1)).concat(list.slice(i + 1, list.len()).expect("tail"))
}

fn main() -> Void! {
    let clock = time.SystemClock.new()
    let rootPem = s(%q)
    let root = x509.parsePem(rootPem).expect("root")
    let leaf = x509.parsePem(s(%q)).expect("leaf")
    let signed = b(%q)
    let forged = malformed(signed)
    let started = clock.now()
    let parsed = match x509.verifyCrlSignature(signed, &root[0]) {
        Ok(_) -> match x509.parseCrl(signed) {
            Ok(list) -> "ok revoked=${list.revoked.len()} revokes=${list.revokes(&leaf[0])}"
            Err(err) -> "error:${err.message()}"
        }
        Err(err) -> "error:${err.message()}"
    }
    print "x509\t${parsed}"
    print "ms\tx509\t${started.elapsedMillis()}"
    let forgedAt = clock.now()
    print "forged\t${detail(sgx.TcbInfo.parseVerified("", "", rootPem, forged, %d))}"
    print "ms\tforged\t${forgedAt.elapsedMillis()}"
    let signedAt = clock.now()
    print "signed\t${code(sgx.TcbInfo.parseVerified("", "", rootPem, signed, %d))}"
    print "ms\tsigned\t${signedAt.elapsedMillis()}"
    let oversizeAt = clock.now()
    print "oversize\t${code(sgx.TcbInfo.parseVerified("", "", rootPem, Bytes.zeros(sgx.maxCrlBytes + 1), %d))}"
    print "ms\toversize\t${oversizeAt.elapsedMillis()}"
}
`

func sgxScaleRun(t *testing.T, out string) (map[string]string, map[string]int) {
	t.Helper()
	results, millis := map[string]string{}, map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) == 3 && parts[0] == "ms" {
			n, err := strconv.Atoi(parts[2])
			if err != nil {
				t.Fatalf("bad timing line %q", line)
			}
			millis[parts[1]] = n
		} else if len(parts) == 2 {
			results[parts[0]] = parts[1]
		} else {
			t.Fatalf("malformed line %q", line)
		}
	}
	return results, millis
}

// TestSgxCrlScale pins that a 20001-entry CRL is read in well under a second
// natively, and that a list with unparseable entries fails on its signature.
func TestSgxCrlScale(t *testing.T) {
	rootPem, leafPem, crl, now := sgxScaleCrl(t)
	at := now.UnixMilli()
	source := fmt.Sprintf(sgxScaleProgram, hex.EncodeToString(rootPem), hex.EncodeToString(leafPem), hex.EncodeToString(crl), at, at, at)
	runSgxScaleProgram(t, source, map[string]string{
		"x509":     fmt.Sprintf("ok revoked=%d revokes=true", sgxCrlEntries),
		"forged":   "error:bad-crl:sgx: root CA CRL: x509: signature does not verify",
		"signed":   "error:bad-signing-chain",
		"oversize": "error:bad-crl",
	}, map[string]int{"x509": 1000, "signed": 1000}, fmt.Sprintf("%d-byte CRL", len(crl)))
}

// runSgxScaleProgram checks the results on both engines; the time limits,
// in milliseconds, apply to native only since VM P-256 alone takes seconds.
func runSgxScaleProgram(t *testing.T, source string, want map[string]string, nativeLimits map[string]int, input string) {
	t.Helper()
	m, root := loadSgxProgram(t, source)
	check := func(engine string, out string, limits map[string]int) {
		results, millis := sgxScaleRun(t, out)
		for key, value := range want {
			if results[key] != value {
				t.Errorf("%s %s = %q, want %q", engine, key, results[key], value)
			}
		}
		t.Logf("%s timings (ms) for a %s: %v", engine, input, millis)
		for key, limit := range limits {
			if millis[key] > limit {
				t.Errorf("%s %s took %d ms, limit %d", engine, key, millis[key], limit)
			}
		}
	}
	var vmOut, vmErr bytes.Buffer
	if code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &vmOut, &vmErr, []string{"main/main.kg"}); err != nil || code != 0 {
		t.Fatalf("vm run: err %v exit %d\n%s", err, code, vmErr.String())
	}
	check("vm", vmOut.String(), nil)
	if testCompiler().CCompiler() == nil {
		t.Skip("native engine skipped: no C compiler")
	}
	exe := filepath.Join(root, "sgx-scale")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = root
	if countsNativeLive() {
		cmd.Env = append(os.Environ(), "KIGUMI_LIVE_REPORT=1")
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("native run: %v\n%s", err, errOut.String())
	}
	if countsNativeLive() {
		if live := nativeLiveLine.FindStringSubmatch(errOut.String()); live == nil || live[1] != "0" {
			t.Fatalf("expected zero native live objects: %s", errOut.String())
		}
		// The RC-checked runtime roughly doubles the time, so its runs check results only.
		nativeLimits = nil
	}
	check("native", out.String(), nativeLimits)
}

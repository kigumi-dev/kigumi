package driver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// SEC-20's primitive-level half: a manifest of checked-in CAVP/Wycheproof
// subsets, compiled once and run on the VM and on the native backend. Each
// vector is a subtest named "<primitive>/<vector>" so a conformance failure
// names both the primitive and the exact vector.
const cryptoCorpus = "../../../testdata/conformance/crypto"

type cryptoManifest struct {
	Schema  int           `json:"schema"`
	Comment string        `json:"comment"`
	Suites  []cryptoSuite `json:"suites"`
}

type cryptoSuite struct {
	Primitive string `json:"primitive"`
	Source    string `json:"source"`
	Path      string `json:"path"`
	Vectors   []cryptoVector
}

type cryptoVectorFile struct {
	Primitive string         `json:"primitive"`
	Source    string         `json:"source"`
	Vectors   []cryptoVector `json:"vectors"`
}

type cryptoVector struct {
	ID         string   `json:"id"`
	Op         string   `json:"op"`
	Alg        string   `json:"alg"`
	Message    string   `json:"message"`
	Key        string   `json:"key"`
	Nonce      string   `json:"nonce"`
	AAD        string   `json:"aad"`
	Plaintext  string   `json:"plaintext"`
	Sealed     string   `json:"sealed"`
	Secret     string   `json:"secret"`
	PeerPublic string   `json:"peerPublic"`
	Public     string   `json:"public"`
	Signature  string   `json:"signature"`
	DER        string   `json:"der"`
	IKM        string   `json:"ikm"`
	Salt       string   `json:"salt"`
	Info       string   `json:"info"`
	Length     int      `json:"length"`
	OutputLen  int      `json:"outputLen"`
	SaltLength int      `json:"saltLength"`
	Roots      []string `json:"roots"`
	Chain      []string `json:"chain"`
	Host       string   `json:"host"`
	NowMillis  int64    `json:"nowMillis"`
	Engines    string   `json:"engines"`
	Expected   *string  `json:"expected"`
}

// loadCryptoCorpus reads the manifest and every vector file, rejecting a
// malformed entry before any program runs. The manifest is the only source of
// suites; a vector file not named there is not exercised.
func loadCryptoCorpus(t *testing.T) *cryptoManifest {
	t.Helper()
	base, err := filepath.Abs(cryptoCorpus)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		t.Fatalf("read crypto manifest: %v", err)
	}
	var m cryptoManifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("parse crypto manifest: %v", err)
	}
	if m.Schema != 1 {
		t.Fatalf("crypto manifest schema = %d, want 1", m.Schema)
	}
	if len(m.Suites) == 0 {
		t.Fatal("crypto manifest has no suites")
	}
	seenPrimitive := map[string]bool{}
	seenVector := map[string]string{}
	usedFiles := map[string]bool{"manifest.json": true}
	for i := range m.Suites {
		suite := &m.Suites[i]
		if suite.Primitive == "" || seenPrimitive[suite.Primitive] {
			t.Fatalf("crypto suite %d: empty or duplicate primitive %q", i, suite.Primitive)
		}
		seenPrimitive[suite.Primitive] = true
		clean := filepath.Clean(suite.Path)
		if clean != suite.Path || suite.Path == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			t.Fatalf("%s: vector path %q must be a relative file inside testdata/conformance/crypto", suite.Primitive, suite.Path)
		}
		usedFiles[clean] = true
		raw, err := os.ReadFile(filepath.Join(base, clean))
		if err != nil {
			t.Fatalf("%s: read vector file: %v", suite.Primitive, err)
		}
		var f cryptoVectorFile
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			t.Fatalf("%s: parse vector file %s: %v", suite.Primitive, suite.Path, err)
		}
		if f.Primitive != suite.Primitive {
			t.Fatalf("%s: vector file declares primitive %q", suite.Primitive, f.Primitive)
		}
		if suite.Source == "" && f.Source == "" {
			t.Fatalf("%s: no source is recorded for the suite", suite.Primitive)
		}
		if len(f.Vectors) == 0 {
			t.Fatalf("%s: vector file has no vectors", suite.Primitive)
		}
		for j := range f.Vectors {
			v := &f.Vectors[j]
			if v.ID == "" {
				t.Fatalf("%s: vector %d has no id", suite.Primitive, j)
			}
			if other, ok := seenVector[v.ID]; ok {
				t.Fatalf("vector id %q is used by both %s and %s", v.ID, other, suite.Primitive)
			}
			seenVector[v.ID] = suite.Primitive
			if !supportedCryptoOp(v.Op) {
				t.Fatalf("%s/%s: unsupported operation %q", suite.Primitive, v.ID, v.Op)
			}
			if v.Engines != "" && v.Engines != "both" && v.Engines != "native" {
				t.Fatalf("%s/%s: engines must be \"both\" or \"native\", got %q", suite.Primitive, v.ID, v.Engines)
			}
			if err := validateCryptoVector(v); err != nil {
				t.Fatalf("%s/%s: %v", suite.Primitive, v.ID, err)
			}
		}
		suite.Vectors = f.Vectors
		if suite.Source == "" {
			suite.Source = f.Source
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read crypto corpus directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || usedFiles[entry.Name()] {
			continue
		}
		t.Errorf("testdata/conformance/crypto/%s is not referenced by manifest.json", entry.Name())
	}
	return &m
}

func supportedCryptoOp(op string) bool {
	switch op {
	case "digest", "shake", "hmac", "hkdf",
		"chacha20poly1305-seal", "chacha20poly1305-open",
		"x25519", "x25519-public", "p256-public", "p256-ecdh",
		"p256-ecdsa-sign", "p256-ecdsa-verify",
		"rsa-pkcs1-sha256-verify", "rsa-pss-sha256-verify",
		"x509-parse", "x509-chain",
		"slhdsa-keygen", "slhdsa-sign", "slhdsa-verify":
		return true
	}
	return false
}

func validateCryptoVector(v *cryptoVector) error {
	if v.Expected == nil {
		return fmt.Errorf("missing expected value")
	}
	switch v.Op {
	case "digest":
		switch v.Alg {
		case "sha256", "sha384", "sha512", "sha3-256", "sha3-512":
		default:
			return fmt.Errorf("unsupported digest %q", v.Alg)
		}
	case "shake":
		if v.Alg != "shake128" && v.Alg != "shake256" {
			return fmt.Errorf("unsupported XOF %q", v.Alg)
		}
		if v.OutputLen <= 0 {
			return fmt.Errorf("shake outputLen must be positive")
		}
	case "hmac", "hkdf":
		if v.Alg != "sha256" && v.Alg != "sha512" {
			return fmt.Errorf("unsupported HMAC/KDF hash %q", v.Alg)
		}
		if v.Op == "hkdf" && v.Length <= 0 {
			return fmt.Errorf("hkdf length must be positive")
		}
		if v.Op == "hkdf" && v.IKM == "" {
			return fmt.Errorf("hkdf needs an IKM")
		}
	case "chacha20poly1305-seal":
		if v.Key == "" || v.Nonce == "" {
			return fmt.Errorf("chacha20poly1305 needs a key and nonce")
		}
	case "chacha20poly1305-open":
		if v.Key == "" || v.Nonce == "" || v.Sealed == "" {
			return fmt.Errorf("chacha20poly1305 open needs a key, nonce and sealed input")
		}
	case "x25519", "x25519-public":
		if v.Secret == "" {
			return fmt.Errorf("x25519 needs a secret")
		}
		if v.Op == "x25519" && v.PeerPublic == "" {
			return fmt.Errorf("x25519 needs a peer public key")
		}
	case "p256-public", "p256-ecdsa-sign":
		if v.Secret == "" {
			return fmt.Errorf("p256 needs a secret")
		}
	case "p256-ecdh":
		if v.Secret == "" || v.PeerPublic == "" {
			return fmt.Errorf("p256 ECDH needs a secret and peer public key")
		}
	case "p256-ecdsa-verify":
		if v.Public == "" || v.Signature == "" {
			return fmt.Errorf("p256 ECDSA needs a public key and signature")
		}
	case "rsa-pkcs1-sha256-verify", "rsa-pss-sha256-verify":
		if v.Key == "" || v.Signature == "" {
			return fmt.Errorf("RSA verification needs a key and signature")
		}
	case "slhdsa-keygen":
		if v.Alg != "sha2-128s" && v.Alg != "sha2-128f" {
			return fmt.Errorf("unsupported SLH-DSA parameter set %q", v.Alg)
		}
		if len(v.Key) != 96 {
			return fmt.Errorf("slhdsa keygen needs 48 bytes of seeds")
		}
		if len(v.Secret) == 0 || len(v.Public) == 0 {
			return fmt.Errorf("slhdsa keygen needs the expected secret and public keys")
		}
	case "slhdsa-sign":
		if v.Alg != "sha2-128s" && v.Alg != "sha2-128f" {
			return fmt.Errorf("unsupported SLH-DSA parameter set %q", v.Alg)
		}
		if len(v.Secret) != 128 || len(v.Public) != 64 || v.Signature == "" {
			return fmt.Errorf("slhdsa sign needs a 64-byte secret key, a 32-byte public key and the expected signature")
		}
	case "slhdsa-verify":
		if v.Alg != "sha2-128s" && v.Alg != "sha2-128f" {
			return fmt.Errorf("unsupported SLH-DSA parameter set %q", v.Alg)
		}
		if len(v.Public) != 64 || v.Signature == "" {
			return fmt.Errorf("slhdsa verify needs a 32-byte public key and a signature")
		}
	case "x509-parse":
		if v.DER == "" {
			return fmt.Errorf("x509 parse needs a DER certificate")
		}
	case "x509-chain":
		if len(v.Roots) == 0 || v.Host == "" || v.NowMillis == 0 {
			return fmt.Errorf("x509 chain needs roots, host and nowMillis")
		}
	}
	return nil
}

// buildCryptoProgram lowers every vector to one Kigumi program. A single
// compilation and run per engine keeps the corpus cheap; the printed lines are
// mapped back to vector ids by parseCryptoResults.
func buildCryptoProgram(t *testing.T, m *cryptoManifest, native bool) string {
	t.Helper()
	imports := map[string]string{
		"hex": "std/encoding/hex",
	}
	for _, suite := range m.Suites {
		for _, v := range suite.Vectors {
			switch v.Op {
			case "digest", "shake":
				switch v.Alg {
				case "sha256":
					imports["sha256"] = "std/crypto/sha256"
				case "sha384", "sha512":
					imports["sha512"] = "std/crypto/sha512"
				case "sha3-256", "sha3-512", "shake128", "shake256":
					imports["sha3"] = "std/crypto/sha3"
				}
			case "hmac":
				imports["hmac"] = "std/crypto/hmac"
				imports["secret"] = "std/secret"
				if v.Alg == "sha256" {
					imports["sha256"] = "std/crypto/sha256"
				} else {
					imports["sha512"] = "std/crypto/sha512"
				}
			case "hkdf":
				imports["hkdf"] = "std/crypto/hkdf"
				imports["secret"] = "std/secret"
				if v.Alg == "sha256" {
					imports["sha256"] = "std/crypto/sha256"
				} else {
					imports["sha512"] = "std/crypto/sha512"
				}
			case "chacha20poly1305-seal", "chacha20poly1305-open":
				imports["chacha20poly1305"] = "std/crypto/chacha20poly1305"
				imports["secret"] = "std/secret"
			case "x25519", "x25519-public":
				imports["x25519"] = "std/crypto/x25519"
				imports["secret"] = "std/secret"
			case "p256-public", "p256-ecdh", "p256-ecdsa-sign", "p256-ecdsa-verify":
				imports["p256"] = "std/crypto/p256"
				if v.Op == "p256-ecdsa-sign" {
					imports["secret"] = "std/secret"
				}
			case "rsa-pkcs1-sha256-verify", "rsa-pss-sha256-verify":
				imports["rsa"] = "std/crypto/rsa"
			case "x509-parse", "x509-chain":
				imports["x509"] = "std/crypto/x509"
			case "slhdsa-keygen", "slhdsa-sign", "slhdsa-verify":
				imports["slhdsa"] = "std/crypto/slhdsa"
			}
		}
	}
	names := make([]string, 0, len(imports))
	for name := range imports {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "import %s from %s\n", name, imports[name])
	}
	b.WriteString(`
fn b(text: String) -> Bytes {
    hex.decode(text).expect("vector hex")
}

fn show(value: Bytes) -> String {
    hex.encode(value)
}
`)
	if _, ok := imports["x509"]; ok {
		b.WriteString(`
fn subjectSummary(cert: x509.Certificate) -> String {
    let subject = cert.subject.commonName || "?"
    let issuer = cert.issuer.commonName || "?"
    "subject=${subject} issuer=${issuer} serial=${hex.encode(cert.serial)} ca=${cert.basicConstraints.ca} dns=${cert.dnsNames.len()}"
}
`)
	}
	b.WriteString("\nfn main() -> Void! {\n")
	idx := 0
	for _, suite := range m.Suites {
		for _, v := range suite.Vectors {
			if !native && v.Engines == "native" {
				continue
			}
			fmt.Fprintf(&b, "    // %s/%s\n", suite.Primitive, v.ID)
			emitCryptoVector(&b, v, idx)
			idx++
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func printHex(b *strings.Builder, id string, idx int, expr string) {
	fmt.Fprintf(b, "    let result%d = show(%s)\n", idx, expr)
	fmt.Fprintf(b, "    print %q\n", id+"\t${result"+fmt.Sprint(idx)+"}")
}

func emitResultBytes(b *strings.Builder, id string, idx int, call string) {
	fmt.Fprintf(b, "    let result%d = match %s {\n", idx, call)
	b.WriteString("        Ok(value) -> show(value)\n")
	b.WriteString("        Err(err) -> \"error:${err.message()}\"\n")
	b.WriteString("    }\n")
	fmt.Fprintf(b, "    print %q\n", id+"\t${result"+fmt.Sprint(idx)+"}")
}

// emitResultSecret prints a Result[secret.SecretBuffer, E] by releasing the
// output at the test boundary; the key and plaintext inputs stay secret.
func emitResultSecret(b *strings.Builder, id string, idx int, call string) {
	fmt.Fprintf(b, "    let result%d = match %s {\n", idx, call)
	b.WriteString("        Ok(value) -> show(unsafe { value.declassify() })\n")
	b.WriteString("        Err(err) -> \"error:${err.message()}\"\n")
	b.WriteString("    }\n")
	fmt.Fprintf(b, "    print %q\n", id+"\t${result"+fmt.Sprint(idx)+"}")
}

func emitResultBool(b *strings.Builder, id string, idx int, call string) {
	fmt.Fprintf(b, "    let result%d = match %s {\n", idx, call)
	b.WriteString("        Ok(value) -> \"${value}\"\n")
	b.WriteString("        Err(err) -> \"error:${err.message()}\"\n")
	b.WriteString("    }\n")
	fmt.Fprintf(b, "    print %q\n", id+"\t${result"+fmt.Sprint(idx)+"}")
}

func emitResultVoid(b *strings.Builder, id string, idx int, call string) {
	fmt.Fprintf(b, "    let result%d = match %s {\n", idx, call)
	b.WriteString("        Ok(_) -> \"ok\"\n")
	b.WriteString("        Err(err) -> \"error:${err.message()}\"\n")
	b.WriteString("    }\n")
	fmt.Fprintf(b, "    print %q\n", id+"\t${result"+fmt.Sprint(idx)+"}")
}

func emitCryptoVector(b *strings.Builder, v cryptoVector, idx int) {
	hexArg := func(s string) string { return fmt.Sprintf("b(%q)", s) }
	switch v.Op {
	case "digest":
		call := ""
		switch v.Alg {
		case "sha256":
			call = "sha256.sum"
		case "sha384":
			call = "sha512.sum384"
		case "sha512":
			call = "sha512.sum"
		case "sha3-256":
			call = "sha3.sum256"
		case "sha3-512":
			call = "sha3.sum512"
		}
		printHex(b, v.ID, idx, fmt.Sprintf("%s(%s)", call, hexArg(v.Message)))
	case "shake":
		call := "sha3.shake128"
		if v.Alg == "shake256" {
			call = "sha3.shake256"
		}
		printHex(b, v.ID, idx, fmt.Sprintf("%s(%s, %d)", call, hexArg(v.Message), v.OutputLen))
	case "hmac":
		hasher := "sha256.SecretHasher"
		if v.Alg == "sha512" {
			hasher = "sha512.SecretHasher"
		}
		call := fmt.Sprintf("unsafe { hmac.sum[%s](&secret.SecretBuffer.fromBytes(%s), &secret.SecretBuffer.fromBytes(%s)).declassify() }",
			hasher, hexArg(v.Key), hexArg(v.Message))
		printHex(b, v.ID, idx, call)
	case "hkdf":
		hasher := "sha256.SecretHasher"
		if v.Alg == "sha512" {
			hasher = "sha512.SecretHasher"
		}
		emitResultSecret(b, v.ID, idx, fmt.Sprintf("hkdf.derive[%s](&secret.SecretBuffer.fromBytes(%s), &secret.SecretBuffer.fromBytes(%s), %s, %d)",
			hasher, hexArg(v.Salt), hexArg(v.IKM), hexArg(v.Info), v.Length))
	case "chacha20poly1305-seal":
		emitResultSecret(b, v.ID, idx, fmt.Sprintf("chacha20poly1305.seal(&secret.SecretBuffer.fromBytes(%s), %s, %s, &secret.SecretBuffer.fromBytes(%s))",
			hexArg(v.Key), hexArg(v.Nonce), hexArg(v.AAD), hexArg(v.Plaintext)))
	case "chacha20poly1305-open":
		emitResultSecret(b, v.ID, idx, fmt.Sprintf("chacha20poly1305.open(&secret.SecretBuffer.fromBytes(%s), %s, %s, %s)",
			hexArg(v.Key), hexArg(v.Nonce), hexArg(v.AAD), hexArg(v.Sealed)))
	case "x25519":
		emitResultSecret(b, v.ID, idx, fmt.Sprintf("x25519.sharedSecret(&secret.SecretBuffer.fromBytes(%s), %s)",
			hexArg(v.Secret), hexArg(v.PeerPublic)))
	case "x25519-public":
		emitResultBytes(b, v.ID, idx, fmt.Sprintf("x25519.publicKey(&secret.SecretBuffer.fromBytes(%s))", hexArg(v.Secret)))
	case "p256-public":
		emitResultBytes(b, v.ID, idx, fmt.Sprintf("p256.publicKey(%s)", hexArg(v.Secret)))
	case "p256-ecdh":
		emitResultBytes(b, v.ID, idx, fmt.Sprintf("p256.sharedSecret(%s, %s)",
			hexArg(v.Secret), hexArg(v.PeerPublic)))
	case "p256-ecdsa-sign":
		emitResultBytes(b, v.ID, idx, fmt.Sprintf("p256.signMessage(&secret.SecretBuffer.fromBytes(%s), %s)",
			hexArg(v.Secret), hexArg(v.Message)))
	case "p256-ecdsa-verify":
		emitResultBool(b, v.ID, idx, fmt.Sprintf("p256.verifyMessage(%s, %s, %s)",
			hexArg(v.Public), hexArg(v.Message), hexArg(v.Signature)))
	case "rsa-pkcs1-sha256-verify":
		key := fmt.Sprintf("pk%d", idx)
		fmt.Fprintf(b, "    let %s = rsa.parsePkcs1(%s).expect(\"rsa public key\")\n", key, hexArg(v.Key))
		emitResultBool(b, v.ID, idx, fmt.Sprintf("rsa.verifyPkcs1v15Sha256(%s, %s, %s)",
			key, hexArg(v.Message), hexArg(v.Signature)))
	case "rsa-pss-sha256-verify":
		key := fmt.Sprintf("pk%d", idx)
		fmt.Fprintf(b, "    let %s = rsa.parsePkcs1(%s).expect(\"rsa public key\")\n", key, hexArg(v.Key))
		salt := "rsa.SaltLength.Auto"
		if v.SaltLength > 0 {
			salt = fmt.Sprintf("rsa.SaltLength.Exact(%d)", v.SaltLength)
		}
		emitResultBool(b, v.ID, idx, fmt.Sprintf("rsa.verifyPssSha256(%s, %s, %s, %s)",
			key, hexArg(v.Message), hexArg(v.Signature), salt))
	case "x509-parse":
		fmt.Fprintf(b, "    let result%d = match x509.parse(%s) {\n", idx, hexArg(v.DER))
		b.WriteString("        Ok(cert) -> subjectSummary(cert)\n")
		b.WriteString("        Err(err) -> \"error:${err.message()}\"\n")
		b.WriteString("    }\n")
		fmt.Fprintf(b, "    print %q\n", v.ID+"\t${result"+fmt.Sprint(idx)+"}")
	case "slhdsa-keygen", "slhdsa-sign", "slhdsa-verify":
		emitSLHDSAVector(b, v, idx)
	case "x509-chain":
		for i, der := range v.Roots {
			fmt.Fprintf(b, "    let anchor%d_%d = x509.parse(%s).expect(\"x509 anchor\")\n", idx, i, hexArg(der))
		}
		anchorItems := make([]string, 0, len(v.Roots))
		for i := range v.Roots {
			anchorItems = append(anchorItems, fmt.Sprintf("anchor%d_%d.clone()", idx, i))
		}
		fmt.Fprintf(b, "    let anchors%d = Array.of(%s)\n", idx, strings.Join(anchorItems, ", "))
		if len(v.Chain) == 0 {
			fmt.Fprintf(b, "    let chain%d = Array.empty[x509.Certificate]()\n", idx)
		} else {
			for i, der := range v.Chain {
				fmt.Fprintf(b, "    let cert%d_%d = x509.parse(%s).expect(\"x509 chain\")\n", idx, i, hexArg(der))
			}
			items := make([]string, 0, len(v.Chain))
			for i := range v.Chain {
				items = append(items, fmt.Sprintf("cert%d_%d.clone()", idx, i))
			}
			fmt.Fprintf(b, "    let chain%d = Array.of(%s)\n", idx, strings.Join(items, ", "))
		}
		emitResultVoid(b, v.ID, idx, fmt.Sprintf("x509.verifyChain(&anchors%d, &chain%d, %q, %d)",
			idx, idx, v.Host, v.NowMillis))
	}
}

func emitSLHDSASuffix(alg string) string {
	if alg == "sha2-128s" {
		return "128s"
	}
	return "128f"
}

func emitSLHDSABytes(b *strings.Builder, id string, idx int, value string) {
	fmt.Fprintf(b, "    let slh%d = b(%q)\n", idx, value)
}

func emitSLHDSAVector(b *strings.Builder, v cryptoVector, idx int) {
	suffix := emitSLHDSASuffix(v.Alg)
	switch v.Op {
	case "slhdsa-keygen":
		emitSLHDSABytes(b, v.ID, idx, v.Key)
		fmt.Fprintf(b, "    let seeds%d = slh%d\n", idx, idx)
		fmt.Fprintf(b, "    let kp%d = slhdsa.keyGen%s(seeds%d.slice(0, 16).expect(\"skSeed\"), seeds%d.slice(16, 32).expect(\"skPrf\"), seeds%d.slice(32, 48).expect(\"pkSeed\")).expect(\"slhdsa keygen\")\n", idx, suffix, idx, idx, idx)
		fmt.Fprintf(b, "    let ok%d = hex.encode(kp%d.secretKey) == %q && hex.encode(kp%d.publicKey) == %q\n", idx, idx, v.Secret, idx, v.Public)
		fmt.Fprintf(b, "    print %q\n", v.ID+"\t${ok"+fmt.Sprint(idx)+"}")
	case "slhdsa-sign":
		emitSLHDSABytes(b, v.ID, idx, v.Secret)
		emitSLHDSABytes(b, v.ID, idx+1, v.Message)
		fmt.Fprintf(b, "    let ctx%d = b(%q)\n", idx, v.AAD)
		fmt.Fprintf(b, "    let sig%d = slhdsa.signDeterministic%s(slh%d, ctx%d, slh%d).expect(\"slhdsa sign\")\n", idx, suffix, idx+1, idx, idx)
		fmt.Fprintf(b, "    let ok%d = hex.encode(sig%d) == %q\n", idx, idx, v.Signature)
		fmt.Fprintf(b, "    print %q\n", v.ID+"\t${ok"+fmt.Sprint(idx)+"}")
	case "slhdsa-verify":
		emitSLHDSABytes(b, v.ID, idx, v.Public)
		emitSLHDSABytes(b, v.ID, idx+1, v.Message)
		emitSLHDSABytes(b, v.ID, idx+2, v.Signature)
		fmt.Fprintf(b, "    let ctx%d = b(%q)\n", idx, v.AAD)
		fmt.Fprintf(b, "    let ok%d = slhdsa.verify%s(slh%d, ctx%d, slh%d, slh%d)\n", idx, suffix, idx+1, idx, idx+2, idx)
		fmt.Fprintf(b, "    print %q\n", v.ID+"\t${ok"+fmt.Sprint(idx)+"}")
	}
}

func parseCryptoResults(t *testing.T, out string) map[string]string {
	t.Helper()
	results := map[string]string{}
	if out == "" {
		return results
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		id, value, ok := strings.Cut(line, "\t")
		if !ok || id == "" {
			t.Fatalf("crypto program emitted a malformed line %q", line)
		}
		if _, dup := results[id]; dup {
			t.Fatalf("crypto program emitted %q twice", id)
		}
		results[id] = value
	}
	return results
}

func runCryptoProgram(t *testing.T, root string, m *driver.Module) map[string]string {
	t.Helper()
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &out, &errOut, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm run: err %v exit %d\nstderr:\n%s", err, code, errOut.String())
	}
	return parseCryptoResults(t, out.String())
}

func buildAndRunCryptoNative(t *testing.T, root string, m *driver.Module) map[string]string {
	t.Helper()
	exe := filepath.Join(root, "crypto-conformance")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = root
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("native run: %v\nstderr:\n%s", err, errOut.String())
	}
	return parseCryptoResults(t, out.String())
}

// loadCryptoProgram writes one generated program into its own module root and
// loads it, so the VM and native runs cannot share lowered state.
func loadCryptoProgram(t *testing.T, source string) (string, *driver.Module) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(root, "main", "main.kg")
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatalf("load generated crypto program (%s): %v", sourcePath, err)
	}
	return root, m
}

func TestCryptoConformance(t *testing.T) {
	corpus := loadCryptoCorpus(t)

	// SLH-DSA signing and key generation at the 128s parameters take minutes on
	// the VM, so those vectors declare `"engines": "native"`; the VM program
	// leaves them out and the native program includes them.
	vmRoot, vmModule := loadCryptoProgram(t, buildCryptoProgram(t, corpus, false))
	vm := runCryptoProgram(t, vmRoot, vmModule)

	var native map[string]string
	if testCompiler().CCompiler() == nil {
		t.Log("native engine skipped: no C compiler")
	} else {
		nativeRoot, nativeModule := loadCryptoProgram(t, buildCryptoProgram(t, corpus, true))
		native = buildAndRunCryptoNative(t, nativeRoot, nativeModule)
	}

	for _, suite := range corpus.Suites {
		for _, v := range suite.Vectors {
			v := v
			t.Run(suite.Primitive+"/"+v.ID, func(t *testing.T) {
				if v.Engines != "native" {
					if got, ok := vm[v.ID]; !ok {
						t.Errorf("%s/%s: vm emitted no result", suite.Primitive, v.ID)
					} else if got != *v.Expected {
						t.Errorf("%s/%s: vm = %q, want %q", suite.Primitive, v.ID, got, *v.Expected)
					}
				}
				if native == nil {
					if v.Engines == "native" {
						t.Logf("%s/%s: native-only vector not run; no C compiler", suite.Primitive, v.ID)
					}
					return
				}
				if got, ok := native[v.ID]; !ok {
					t.Errorf("%s/%s: native emitted no result", suite.Primitive, v.ID)
				} else if got != *v.Expected {
					t.Errorf("%s/%s: native = %q, want %q", suite.Primitive, v.ID, got, *v.Expected)
				}
			})
		}
	}
}

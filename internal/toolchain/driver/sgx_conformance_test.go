package driver_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// Every malformed or forged vector expects an exact error code, so a parser
// that starts accepting it fails here.
const sgxCorpus = "../../../testdata/conformance/sgx"

type sgxManifest struct {
	Schema  int        `json:"schema"`
	Comment string     `json:"comment"`
	Suites  []sgxSuite `json:"suites"`
}

type sgxSuite struct {
	Primitive string            `json:"primitive"`
	Source    string            `json:"source"`
	Path      string            `json:"path"`
	Blobs     map[string]string `json:"-"`
	Vectors   []sgxVector       `json:"vectors"`
}

type sgxVectorFile struct {
	Primitive string            `json:"primitive"`
	Source    string            `json:"source"`
	Blobs     map[string]string `json:"blobs"`
	Vectors   []sgxVector       `json:"vectors"`
}

type sgxVector struct {
	ID            string         `json:"id"`
	Op            string         `json:"op"`
	Quote         string         `json:"quote"`
	Hex           string         `json:"hex"`
	Hex2          string         `json:"hex2"`
	Anchors       string         `json:"anchors"`
	Now           int64          `json:"now"`
	ReportData    string         `json:"reportData"`
	Measurement   string         `json:"measurement"`
	CpuSvn        string         `json:"cpuSvn"`
	PceSvn        uint16         `json:"pceSvn"`
	Args          []string       `json:"args"`
	Collateral    *sgxCollateral `json:"collateral"`
	Accept        string         `json:"accept"`
	MinEvaluation uint32         `json:"minEvaluation"`
	Expected      string         `json:"expected"`
}

type sgxCollateral struct {
	TcbInfo    string `json:"tcbInfo"`
	TcbChain   string `json:"tcbChain"`
	QeIdentity string `json:"qeIdentity"`
	QeChain    string `json:"qeChain"`
	PckCrl     string `json:"pckCrl"`
	RootCrl    string `json:"rootCrl"`
}

func loadSgxCorpus(t *testing.T) *sgxManifest {
	t.Helper()
	base, err := filepath.Abs(sgxCorpus)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		t.Fatalf("read sgx manifest: %v", err)
	}
	var m sgxManifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("parse sgx manifest: %v", err)
	}
	if m.Schema != 1 || len(m.Suites) == 0 {
		t.Fatalf("sgx manifest schema = %d, suites = %d", m.Schema, len(m.Suites))
	}
	seenPrimitive := map[string]bool{}
	seenVector := map[string]string{}
	usedFiles := map[string]bool{"manifest.json": true}
	for i := range m.Suites {
		suite := &m.Suites[i]
		if suite.Primitive == "" || seenPrimitive[suite.Primitive] {
			t.Fatalf("sgx suite %d: empty or duplicate primitive %q", i, suite.Primitive)
		}
		seenPrimitive[suite.Primitive] = true
		clean := filepath.Clean(suite.Path)
		if clean != suite.Path || suite.Path == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			t.Fatalf("%s: vector path %q must be a relative file inside testdata/conformance/sgx", suite.Primitive, suite.Path)
		}
		usedFiles[clean] = true
		raw, err := os.ReadFile(filepath.Join(base, clean))
		if err != nil {
			t.Fatalf("%s: read vector file: %v", suite.Primitive, err)
		}
		var f sgxVectorFile
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&f); err != nil {
			t.Fatalf("%s: parse vector file %s: %v", suite.Primitive, suite.Path, err)
		}
		if f.Primitive != suite.Primitive {
			t.Fatalf("%s: vector file declares primitive %q", suite.Primitive, f.Primitive)
		}
		if f.Source == "" {
			t.Fatalf("%s: no source is recorded", suite.Primitive)
		}
		if len(f.Vectors) == 0 {
			t.Fatalf("%s: vector file has no vectors", suite.Primitive)
		}
		for j := range f.Vectors {
			v := &f.Vectors[j]
			if v.ID == "" || v.Expected == "" {
				t.Fatalf("%s: vector %d needs an id and expected value", suite.Primitive, j)
			}
			key := suite.Primitive + "/" + v.ID
			if other, ok := seenVector[key]; ok {
				t.Fatalf("vector id %q is used by both %s and %s", key, other, suite.Primitive)
			}
			seenVector[key] = suite.Primitive
			if err := validateSgxVector(v, f.Blobs); err != nil {
				t.Fatalf("%s/%s: %v", suite.Primitive, v.ID, err)
			}
		}
		suite.Vectors = f.Vectors
		suite.Blobs = f.Blobs
		if suite.Source == "" {
			suite.Source = f.Source
		}
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read sgx corpus directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || usedFiles[entry.Name()] {
			continue
		}
		t.Errorf("testdata/conformance/sgx/%s is not referenced by manifest.json", entry.Name())
	}
	return &m
}

func validateSgxVector(v *sgxVector, blobs map[string]string) error {
	switch v.Op {
	case "sgx-parse", "sgx-signature", "sgx-qe", "sgx-root", "sgx-pck-extension":
		if v.Hex == "" {
			return fmt.Errorf("needs input bytes")
		}
	case "sgx-pck":
		if v.Hex == "" || v.Anchors == "" {
			return fmt.Errorf("needs a chain and anchors")
		}
	case "sgx-qe-binding":
		if v.Hex == "" {
			return fmt.Errorf("needs an attestation public key")
		}
	case "sgx-crl":
		if v.Hex == "" || v.Anchors == "" || v.Hex2 == "" {
			return fmt.Errorf("needs a CRL, its issuer and a certificate")
		}
	case "sgx-converge":
		if len(v.Args) != 2 {
			return fmt.Errorf("needs a platform and a QE status")
		}
	case "sgx-verify", "sgx-tcb", "sgx-qe-identity", "sgx-collateral-auth":
		return validateCollateralVector(v, blobs)
	default:
		return fmt.Errorf("unsupported operation %q", v.Op)
	}
	return nil
}

func emitSgxVector(b *strings.Builder, v sgxVector, idx int, blob func(string) string) {
	hexArg := func(field string) string { return fmt.Sprintf("b(%q)", field) }
	emit := func(body string) {
		fmt.Fprintf(b, "    let result%d = %s\n", idx, body)
	}
	if body, ok := emitCollateralVector(v, blob); ok {
		emit(body)
		return
	}
	switch v.Op {
	case "sgx-qe-binding":
		emit(fmt.Sprintf(`match sgx.qeReportData(%s, %s) {
        Ok(data) -> hex.encode(data)
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), hexArg(v.Hex2)))
	case "sgx-parse":
		emit(fmt.Sprintf(`match sgx.Quote.parse(%s) {
        Ok(q) -> "ok mr=${hex.encode(q.report.mrEnclave)} signer=${hex.encode(q.report.mrSigner)} isv=${q.report.isvProdId}/${q.report.isvSvn} debug=${p(q.report.debug())} qeSvn=${q.header.qeSvn}"
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "sgx-signature":
		emit(fmt.Sprintf(`match sgx.Quote.parse(%s) {
        Ok(q) -> match q.verifySignature() {
            Ok(_) -> "ok"
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "sgx-qe":
		emit(fmt.Sprintf(`match sgx.Quote.parse(%s) {
        Ok(q) -> match sgx.qeCertificationOf(&q) {
            Ok(qe) -> match sgx.verifyQe(&qe, q.attestationKey) {
                Ok(verified) -> "ok signer=${hex.encode(verified.report.mrSigner)} isvSvn=${verified.report.isvSvn} certs=${verified.chain.certificates.len()}"
                Err(err) -> "error:${err.code.name()}"
            }
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "sgx-pck":
		emit(fmt.Sprintf(`match sgx.parseChain(%s) {
        Ok(chain) -> match sgx.parseAnchors(s(%q)) {
            Ok(anchors) -> match chain.verify(&anchors, %d) {
                Ok(_) -> "ok certs=${chain.certificates.len()}"
                Err(err) -> "error:${err.code.name()}"
            }
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), v.Anchors, v.Now))
	}
}

func parseSgxResults(t *testing.T, out string) map[string]string {
	t.Helper()
	results := map[string]string{}
	if out == "" {
		return results
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		id, value, ok := strings.Cut(line, "\t")
		if !ok || id == "" {
			t.Fatalf("sgx program emitted a malformed line %q", line)
		}
		if _, dup := results[id]; dup {
			t.Fatalf("sgx program emitted %q twice", id)
		}
		results[id] = value
	}
	return results
}

func TestSgxConformance(t *testing.T) {
	corpus := loadSgxCorpus(t)
	vm := runSgxShardsOnVM(t, corpus, 4)
	native := runSgxNative(t, corpus)
	for _, suite := range corpus.Suites {
		for _, v := range suite.Vectors {
			key := suite.Primitive + "/" + v.ID
			t.Run(key, func(t *testing.T) {
				if got, ok := vm[key]; !ok {
					t.Errorf("vm emitted no result")
				} else if got != v.Expected {
					t.Errorf("vm = %q, want %q", got, v.Expected)
				}
				if native == nil {
					return
				}
				if got, ok := native[key]; !ok {
					t.Errorf("native emitted no result")
				} else if got != v.Expected {
					t.Errorf("native = %q, want %q", got, v.Expected)
				}
			})
		}
	}
}

func loadSgxProgram(t *testing.T, source string) (*driver.Module, string) {
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
		t.Fatalf("load generated sgx program (%s): %v", sourcePath, err)
	}
	return m, root
}

func runSgxNative(t *testing.T, corpus *sgxManifest) map[string]string {
	t.Helper()
	if testCompiler().CCompiler() == nil {
		t.Log("native engine skipped: no C compiler")
		return nil
	}
	m, root := loadSgxProgram(t, buildSgxProgram(corpus))
	exe := filepath.Join(root, "sgx-conformance")
	var buildErr bytes.Buffer
	ok, err := testBuild(m, exe, &buildErr)
	if err != nil || !ok {
		t.Fatalf("native build: %v\n%s", err, buildErr.String())
	}
	cmd := exec.Command(exe)
	cmd.Dir = root
	if countsNativeLive() {
		cmd.Env = append(os.Environ(), "KIGUMI_LIVE_REPORT=1")
	}
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("native run: %v\nstderr:\n%s", err, errOut.String())
	}
	stderr := errOut.String()
	if countsNativeLive() {
		live := nativeLiveLine.FindStringSubmatch(stderr)
		if live == nil || live[1] != "0" {
			t.Fatalf("expected zero native live objects: %s", stderr)
		}
		stderr = nativeLiveLine.ReplaceAllString(stderr, "")
	}
	if stderr != "" {
		t.Fatalf("unexpected native stderr: %s", stderr)
	}
	return parseSgxResults(t, out.String())
}

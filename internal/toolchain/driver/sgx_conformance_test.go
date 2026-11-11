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

// SEC-14's SGX DCAP Tier 0 half: deterministic SGX quotes, QE reports, P-256
// PCK chains and TCB/QE identity collateral compiled once and run on the VM
// and the native backend. Every malformed or forged vector expects an exact
// error code, so a parser that starts accepting it fails here.
const sgxCorpus = "../../../testdata/conformance/sgx"

type sgxManifest struct {
	Schema  int        `json:"schema"`
	Comment string     `json:"comment"`
	Suites  []sgxSuite `json:"suites"`
}

type sgxSuite struct {
	Primitive string      `json:"primitive"`
	Source    string      `json:"source"`
	Path      string      `json:"path"`
	Vectors   []sgxVector `json:"vectors"`
}

type sgxVectorFile struct {
	Primitive string      `json:"primitive"`
	Source    string      `json:"source"`
	Vectors   []sgxVector `json:"vectors"`
}

type sgxVector struct {
	ID         string `json:"id"`
	Op         string `json:"op"`
	Hex        string `json:"hex"`
	Hex2       string `json:"hex2"`
	Anchors    string `json:"anchors"`
	Now        int64  `json:"now"`
	TcbInfo    string `json:"tcbInfo"`
	QeIdentity string `json:"qeIdentity"`
	ReportData string `json:"reportData"`
	CpuSvn     string `json:"cpuSvn"`
	PceSvn     uint16 `json:"pceSvn"`
	Expected   string `json:"expected"`
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
			if err := validateSgxVector(v); err != nil {
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

func validateSgxVector(v *sgxVector) error {
	switch v.Op {
	case "sgx-parse", "sgx-signature", "sgx-qe", "sgx-verify":
		if v.Hex == "" {
			return fmt.Errorf("needs a quote")
		}
	case "sgx-pck":
		if v.Hex == "" || v.Anchors == "" {
			return fmt.Errorf("needs a chain and anchors")
		}
	case "sgx-tcb":
		if v.Hex == "" || v.CpuSvn == "" {
			return fmt.Errorf("needs TCB Info and a CPU SVN")
		}
	default:
		return fmt.Errorf("unsupported operation %q", v.Op)
	}
	return nil
}

func buildSgxProgram(m *sgxManifest) string {
	var prog strings.Builder
	prog.WriteString(`import sgx from std/cvm/sgx
import hex from std/encoding/hex

fn b(text: String) -> Bytes {
    hex.decode(text).expect("vector hex")
}

fn s(text: String) -> String {
    String.fromBytes(b(text)).expect("vector text")
}

fn p(flag: Bool) -> String {
    if flag { "true" } else { "false" }
}

fn tcbName(t: sgx.TcbEvaluation?) -> String {
    match t {
        Some(e) -> e.status.name()
        None -> "none"
    }
}

fn qeName(t: sgx.QeIdentityEvaluation?) -> String {
    match t {
        Some(e) -> e.status.name()
        None -> "none"
    }
}

fn result(r: sgx.VerificationResult) -> String {
    "status=${r.status.name()} advisories=${r.advisoryIds.len()} tcb=${tcbName(r.tcb)} qe=${qeName(r.qe)}"
}

fn main() -> Void! {
`)
	idx := 0
	for _, suite := range m.Suites {
		for _, v := range suite.Vectors {
			fmt.Fprintf(&prog, "\n    // %s/%s\n", suite.Primitive, v.ID)
			emitSgxVector(&prog, v, idx)
			fmt.Fprintf(&prog, "    print %q\n", suite.Primitive+"/"+v.ID+"\t${result"+fmt.Sprint(idx)+"}")
			idx++
		}
	}
	prog.WriteString("}\n")
	return prog.String()
}

func emitSgxVector(b *strings.Builder, v sgxVector, idx int) {
	hexArg := func(field string) string { return fmt.Sprintf("b(%q)", field) }
	emit := func(body string) {
		fmt.Fprintf(b, "    let result%d = %s\n", idx, body)
	}
	switch v.Op {
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
	case "sgx-tcb":
		emit(fmt.Sprintf(`match sgx.TcbInfo.parse(s(%q)) {
        Ok(info) -> {
            let evaluation = info.evaluate(%s, %d)
            "ok status=${evaluation.status.name()} matched=${p(evaluation.matched)} advisories=${evaluation.advisoryIds.len()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, v.Hex, hexArg(v.CpuSvn), v.PceSvn))
	case "sgx-verify":
		var body strings.Builder
		fmt.Fprintf(&body, `{
        let mut policy = sgx.Policy.production(s(%q), %d)
        let mut policyError = ""
`, v.Anchors, v.Now)
		if v.ReportData != "" {
			fmt.Fprintf(&body, "        policy.requireReportData = Some(%s)\n", hexArg(v.ReportData))
		}
		if v.Hex2 != "" {
			fmt.Fprintf(&body, "        policy.requireFmspc = Some(%s)\n", hexArg(v.Hex2))
		}
		if v.TcbInfo != "" {
			fmt.Fprintf(&body, `        match sgx.TcbInfo.parse(s(%q)) {
            Ok(info) -> { policy.tcbInfo = Some(info) }
            Err(err) -> { policyError = "error:${err.code.name()}" }
        }
`, v.TcbInfo)
		}
		if v.QeIdentity != "" {
			fmt.Fprintf(&body, `        match sgx.QeIdentity.parse(s(%q)) {
            Ok(identity) -> { policy.qeIdentity = Some(identity) }
            Err(err) -> { policyError = "error:${err.code.name()}" }
        }
`, v.QeIdentity)
		}
		fmt.Fprintf(&body, `        if policyError != "" {
            policyError
        } else {
            match sgx.verify(%s, &policy) {
                Ok(verified) -> result(verified)
                Err(err) -> "error:${err.code.name()}"
            }
        }
    }`, hexArg(v.Hex))
		emit(body.String())
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
	source := buildSgxProgram(corpus)

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

	var vmOut, vmErr bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &vmOut, &vmErr, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm run: err %v exit %d\nstderr:\n%s", err, code, vmErr.String())
	}
	vm := parseSgxResults(t, vmOut.String())

	var native map[string]string
	if testCompiler().CCompiler() == nil {
		t.Log("native engine skipped: no C compiler")
	} else {
		exe := filepath.Join(root, "sgx-conformance")
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
		native = parseSgxResults(t, out.String())
	}

	for _, suite := range corpus.Suites {
		for _, v := range suite.Vectors {
			v := v
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

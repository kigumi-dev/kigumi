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

// SEC-15/16's TDX and SNP structure half: deterministic TDREPORT, v4 quote,
// RTMR event-log and SNP ID-block vectors compiled once and run on the VM and
// the native backend. Malformed and forged vectors expect the exact error
// code, so a parser that starts accepting them fails here.
const cvmCorpus = "../../../testdata/conformance/cvm"

type cvmManifest struct {
	Schema  int        `json:"schema"`
	Comment string     `json:"comment"`
	Suites  []cvmSuite `json:"suites"`
}

type cvmSuite struct {
	Primitive string      `json:"primitive"`
	Source    string      `json:"source"`
	Path      string      `json:"path"`
	Vectors   []cvmVector `json:"vectors"`
}

type cvmVectorFile struct {
	Primitive string      `json:"primitive"`
	Source    string      `json:"source"`
	Vectors   []cvmVector `json:"vectors"`
}

type cvmVector struct {
	ID              string   `json:"id"`
	Op              string   `json:"op"`
	Hex             string   `json:"hex"`
	Hex2            string   `json:"hex2"`
	MrTd            string   `json:"mrTd"`
	Log             string   `json:"log"`
	Rtmrs           []string `json:"rtmrs"`
	IdKeyDigest     string   `json:"idKeyDigest"`
	AuthorKeyDigest string   `json:"authorKeyDigest"`
	Debug           bool     `json:"debug"`
	Nonce           string   `json:"nonce"`
	Rim             string   `json:"rim"`
	Rems            []string `json:"rems"`
	Expected        string   `json:"expected"`
}

func loadCvmCorpus(t *testing.T) *cvmManifest {
	t.Helper()
	base, err := filepath.Abs(cvmCorpus)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		t.Fatalf("read cvm manifest: %v", err)
	}
	var m cvmManifest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("parse cvm manifest: %v", err)
	}
	if m.Schema != 1 || len(m.Suites) == 0 {
		t.Fatalf("cvm manifest schema = %d, suites = %d", m.Schema, len(m.Suites))
	}
	seenPrimitive := map[string]bool{}
	seenVector := map[string]string{}
	usedFiles := map[string]bool{"manifest.json": true}
	for i := range m.Suites {
		suite := &m.Suites[i]
		if suite.Primitive == "" || seenPrimitive[suite.Primitive] {
			t.Fatalf("cvm suite %d: empty or duplicate primitive %q", i, suite.Primitive)
		}
		seenPrimitive[suite.Primitive] = true
		clean := filepath.Clean(suite.Path)
		if clean != suite.Path || suite.Path == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			t.Fatalf("%s: vector path %q must be a relative file inside testdata/conformance/cvm", suite.Primitive, suite.Path)
		}
		usedFiles[clean] = true
		raw, err := os.ReadFile(filepath.Join(base, clean))
		if err != nil {
			t.Fatalf("%s: read vector file: %v", suite.Primitive, err)
		}
		var f cvmVectorFile
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
			if err := validateCvmVector(v); err != nil {
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
		t.Fatalf("read cvm corpus directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || usedFiles[entry.Name()] {
			continue
		}
		t.Errorf("testdata/conformance/cvm/%s is not referenced by manifest.json", entry.Name())
	}
	return &m
}

func validateCvmVector(v *cvmVector) error {
	switch v.Op {
	case "tdx-report", "tdx-quote", "tdx-quote-qe", "attest-report", "attest-report-unsigned", "attest-quote",
		"report-data", "report-data-nonce":
		if v.Hex == "" {
			return fmt.Errorf("needs a hex input")
		}
	case "attest-quote-log":
		if v.Hex == "" || v.Log == "" {
			return fmt.Errorf("needs a quote and an event log")
		}
	case "eventlog-replay":
		if v.Hex == "" {
			return fmt.Errorf("needs an event log")
		}
	case "eventlog-verify":
		if v.Hex == "" || len(v.Rtmrs) != 4 {
			return fmt.Errorf("needs an event log and four RTMR values")
		}
	case "attest-quote-rtmr":
		if v.Hex == "" || len(v.Rtmrs) != 1 {
			return fmt.Errorf("needs a quote and one RTMR value")
		}
	case "cca-token", "cca-collection", "cca-collection-binding", "attest-cca",
		"attest-cca-reject-platform", "attest-cca-reject-realm", "attest-cca-no-platform":
		if v.Hex == "" {
			return fmt.Errorf("needs a CCA token or collection")
		}
	case "attest-cca-separated":
		if v.Hex == "" || v.Hex2 == "" {
			return fmt.Errorf("needs a realm token and a platform token")
		}
	case "evidence-snp":
	case "idblock":
		if v.Hex == "" || v.Hex2 == "" {
			return fmt.Errorf("needs an ID block and an ID auth block")
		}
	default:
		return fmt.Errorf("unsupported operation %q", v.Op)
	}
	return nil
}

func buildCvmProgram(m *cvmManifest) string {
	names := []string{"attest", "cca", "hex", "sha512", "snp", "tdx"}
	imports := map[string]string{
		"attest": "std/cvm/attest",
		"cca":    "std/cvm/cca",
		"hex":    "std/encoding/hex",
		"sha512": "std/crypto/sha512",
		"snp":    "std/cvm/snp",
		"tdx":    "std/cvm/tdx",
	}
	sort.Strings(names)
	var prog strings.Builder
	for _, name := range names {
		fmt.Fprintf(&prog, "import %s from %s\n", name, imports[name])
	}
	prog.WriteString(`
fn b(text: String) -> Bytes {
    hex.decode(text).expect("vector hex")
}

fn p(flag: Bool) -> String {
    if flag { "true" } else { "false" }
}

fn policyFor(mrTd: String) -> attest.TdxPolicy {
    let mut policy = attest.TdxPolicy.production()
    policy.allowUnauthenticated = true
    if mrTd != "" {
        policy.requireMrTd = Some(b(mrTd))
    }
    policy
}

fn quoteResult(q: tdx.Quote, prefix: String) -> String {
    "${prefix} mrtd=${hex.encode(q.report.mrTd)} debug=${p((q.report.attributes & 1) != 0)} rtmr0=${hex.encode(q.report.rtmr[0])}"
}

fn reg(rtmr: &tdx.Rtmr, index: usize) -> String {
    hex.encode(rtmr.value(index).expect("rtmr"))
}

fn snpCode(err: snp.Error) -> String {
    match err {
        snp.Error.Truncated(_) -> "truncated"
        snp.Error.BadSize(_, _) -> "bad-size"
        snp.Error.UnsupportedVersion(_) -> "unsupported-version"
        snp.Error.UnsupportedAlgorithm(_) -> "unsupported-algorithm"
        snp.Error.ReservedNonZero(_) -> "reserved-nonzero"
        snp.Error.Malformed(_) -> "malformed"
        snp.Error.Unsupported(_) -> "unsupported"
        snp.Error.Unsigned -> "unsigned"
        snp.Error.Forged(_) -> "forged"
    }
}

fn ccaCode(err: cca.Error) -> String {
    match err {
        cca.Error.Truncated(_) -> "truncated"
        cca.Error.BadSize(_, _) -> "bad-size"
        cca.Error.UnsupportedVersion(_) -> "unsupported-version"
        cca.Error.UnsupportedAlgorithm(_) -> "unsupported-algorithm"
        cca.Error.Malformed(_) -> "malformed"
        cca.Error.Unsupported(_) -> "unsupported"
        cca.Error.Forged(_) -> "forged"
        cca.Error.PolicyFailure(_) -> "policy"
    }
}

fn attestCode(err: attest.Error) -> String {
    match err {
        attest.Error.Cca(error) -> "cca-${ccaCode(error)}"
        attest.Error.Snp(_) -> "snp"
        attest.Error.PolicyFailure(_) -> "policy"
        attest.Error.Unsupported(_) -> "unsupported"
        attest.Error.Forged(_) -> "forged"
    }
}

fn verifyCcaEs256(algorithm: Int, key: &cca.CoseKey, signed: Bytes, signature: Bytes) -> Result[Bool, cca.Error] {
    cca.verifyEs256(algorithm, key, signed, signature)
}

fn rejectCcaEs256(algorithm: Int, key: &cca.CoseKey, signed: Bytes, signature: Bytes) -> Result[Bool, cca.Error] {
    let _ = algorithm
    let _ = key
    let _ = signed
    let _ = signature
    Ok(false)
}

fn acceptPlatform(algorithm: Int, signed: Bytes, signature: Bytes) -> Result[Bool, cca.Error] {
    Ok(algorithm == -7 && signed.len() > 0 && signature.len() == 64)
}

fn rejectPlatform(algorithm: Int, signed: Bytes, signature: Bytes) -> Result[Bool, cca.Error] {
    let _ = algorithm
    let _ = signed
    let _ = signature
    Ok(false)
}

fn ccaDigest(claims: &cca.Claims) -> String {
    match cca.claimsPublicKeyDigest(claims) {
        Some(value) -> hex.encode(value)
        None -> "none"
    }
}
`)
	prog.WriteString("\nfn main() -> Void! {\n")
	idx := 0
	for _, suite := range m.Suites {
		for _, v := range suite.Vectors {
			fmt.Fprintf(&prog, "\n    // %s/%s\n", suite.Primitive, v.ID)
			emitCvmVector(&prog, v, idx)
			fmt.Fprintf(&prog, "    print %q\n", suite.Primitive+"/"+v.ID+"\t${result"+fmt.Sprint(idx)+"}")
			idx++
		}
	}
	prog.WriteString("}\n")
	return prog.String()
}

func emitCvmVector(b *strings.Builder, v cvmVector, idx int) {
	hexArg := func(s string) string { return fmt.Sprintf("b(%q)", s) }
	emit := func(body string) {
		fmt.Fprintf(b, "    let result%d = %s\n", idx, body)
	}
	switch v.Op {
	case "tdx-report":
		emit(fmt.Sprintf(`match tdx.TdReport.parse(%s) {
        Ok(report) -> {
            let mrtd = hex.encode(report.tdInfo.mrTd)
            let debug = (report.tdInfo.attributes & 1) != 0
            match report.checkIntegrity() {
                Ok(_) -> "ok mrtd=${mrtd} debug=${p(debug)} integrity=ok"
                Err(err) -> "error:${err.code.name()}"
            }
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "attest-report-unsigned":
		emit(fmt.Sprintf(`match tdx.TdReport.parse(%s) {
        Ok(report) -> match attest.verifyTdxReport(report, &attest.TdxPolicy.production()) {
            Ok(claims) -> "ok authenticated=${p(claims.authenticated)} debug=${p(claims.debug)}"
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "tdx-quote":
		emit(fmt.Sprintf(`match tdx.Quote.parse(%s) {
        Ok(q) -> match q.verifySignature() {
            Ok(_) -> quoteResult(q, "ok")
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "tdx-quote-qe":
		emit(fmt.Sprintf(`match tdx.Quote.parse(%s) {
        Ok(q) -> match q.verifyQeSignature() {
            Ok(_) -> "ok"
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "attest-report":
		emit(fmt.Sprintf(`match tdx.TdReport.parse(%s) {
        Ok(report) -> {
            let policy = policyFor(%q)
            match attest.verifyTdxReport(report, &policy) {
                Ok(claims) -> "ok authenticated=${p(claims.authenticated)} debug=${p(claims.debug)}"
                Err(err) -> "error:${err.code.name()}"
            }
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), v.MrTd))
	case "attest-quote":
		emit(fmt.Sprintf(`match tdx.Quote.parse(%s) {
        Ok(q) -> {
            let policy = policyFor(%q)
            match attest.verifyTdxQuote(q, &policy) {
                Ok(claims) -> "ok authenticated=${p(claims.authenticated)} debug=${p(claims.debug)}"
                Err(err) -> "error:${err.code.name()}"
            }
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), v.MrTd))
	case "attest-quote-log":
		emit(fmt.Sprintf(`match tdx.Quote.parse(%s) {
        Ok(q) -> match tdx.EventLog.parse(%s) {
            Ok(log) -> {
                let policy = policyFor(%q)
                match attest.verifyTdxQuoteWithLog(q, log, &policy) {
                    Ok(claims) -> "ok authenticated=${p(claims.authenticated)}"
                    Err(err) -> "error:${err.code.name()}"
                }
            }
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), hexArg(v.Log), v.MrTd))
	case "eventlog-replay":
		emit(fmt.Sprintf(`match tdx.EventLog.parse(%s) {
        Ok(log) -> match log.replay() {
            Ok(rtmr) -> "rtmr0=${reg(&rtmr, 0)} rtmr1=${reg(&rtmr, 1)} rtmr2=${reg(&rtmr, 2)} rtmr3=${reg(&rtmr, 3)}"
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "eventlog-verify":
		values := make([]string, 0, 4)
		for _, h := range v.Rtmrs {
			values = append(values, hexArg(h))
		}
		emit(fmt.Sprintf(`match tdx.EventLog.parse(%s) {
        Ok(log) -> match log.replay() {
            Ok(rtmr) -> {
                let expected = Array.of(%s)
                match rtmr.verify(&expected) {
                    Ok(_) -> "ok"
                    Err(err) -> "error:${err.code.name()}"
                }
            }
            Err(err) -> "error:${err.code.name()}"
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), strings.Join(values, ", ")))
	case "report-data":
		emit(fmt.Sprintf(`match attest.ReportData.of(%s) {
        Ok(rd) -> "ok ${hex.encode(rd.bytes)}"
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex)))
	case "report-data-nonce":
		emit(fmt.Sprintf(`"ok ${hex.encode(attest.ReportData.fromNonce(%s).bytes)}"`, hexArg(v.Hex)))
	case "attest-quote-rtmr":
		emit(fmt.Sprintf(`match tdx.Quote.parse(%s) {
        Ok(q) -> {
            let mut policy = attest.TdxPolicy.production()
            policy.requireRtmr = Array.of(attest.RtmrRule {
                index: 0
                value: b(%q)
            })
            match attest.verifyTdxQuote(q, &policy) {
                Ok(_) -> "ok"
                Err(err) -> "error:${err.code.name()}"
            }
        }
        Err(err) -> "error:${err.code.name()}"
    }`, hexArg(v.Hex), v.Rtmrs[0]))
	case "cca-token":
		emit(fmt.Sprintf(`match cca.Token.parse(%s) {
        Ok(token) -> match cca.checkClaimsProfile(&token.claims) {
            Ok(_) -> match cca.verifyToken(&token, verifyCcaEs256) {
                Ok(_) -> "ok alg=${token.claims.hashAlgorithm.name()} rim=${hex.encode(token.claims.initialMeasurement)} rem0=${hex.encode(token.claims.extensibleMeasurements[0])} mec=${token.claims.mecPolicy} lfa=${token.claims.lfaPolicy || 9} digest=${ccaDigest(&token.claims)}"
                Err(err) -> "error:${ccaCode(err)}"
            }
            Err(err) -> "error:${ccaCode(err)}"
        }
        Err(err) -> "error:${ccaCode(err)}"
    }`, hexArg(v.Hex)))
	case "cca-collection":
		emit(fmt.Sprintf(`match cca.Collection.parse(%s) {
        Ok(collection) -> "ok platform=${p(collection.platform is Some(_))} rim=${hex.encode(collection.realm.claims.initialMeasurement)} binding=${ccaDigest(&collection.realm.claims)}"
        Err(err) -> "error:${ccaCode(err)}"
    }`, hexArg(v.Hex)))
	case "cca-collection-binding":
		emit(fmt.Sprintf(`match cca.Collection.parse(%s) {
        Ok(collection) -> match cca.verifyToken(&collection.realm, verifyCcaEs256) {
            Ok(_) -> match collection.platform {
                Some(platform) -> match cca.verifyCoseSign1Keyless(&platform.cose, acceptPlatform) {
                    Ok(_) -> match cca.verifyCollectionBinding(&collection) {
                        Ok(_) -> "ok binding=${ccaDigest(&collection.realm.claims)}"
                        Err(err) -> "error:${ccaCode(err)}"
                    }
                    Err(err) -> "error:${ccaCode(err)}"
                }
                None -> "error:unsupported"
            }
            Err(err) -> "error:${ccaCode(err)}"
        }
        Err(err) -> "error:${ccaCode(err)}"
    }`, hexArg(v.Hex)))
	case "attest-cca", "attest-cca-reject-platform", "attest-cca-reject-realm",
		"attest-cca-no-platform", "attest-cca-separated":
		nonceLine := ""
		if v.Nonce != "" {
			nonceLine += fmt.Sprintf("    if %q != \"\" { policy.nonce = Some(b(%q)) }\n", v.Nonce, v.Nonce)
		}
		rimLine := ""
		if v.Rim != "" {
			rimLine += fmt.Sprintf("    if %q != \"\" { policy.initialMeasurement = Some(b(%q)) }\n", v.Rim, v.Rim)
		}
		remLine := ""
		if len(v.Rems) > 0 {
			items := make([]string, 0, len(v.Rems))
			for _, h := range v.Rems {
				items = append(items, hexArg(h))
			}
			remLine = fmt.Sprintf("    policy.extensibleMeasurements = Some(Array.of(%s))\n", strings.Join(items, ", "))
		}
		realmVerifier := "verifyCcaEs256"
		if v.Op == "attest-cca-reject-realm" {
			realmVerifier = "rejectCcaEs256"
		}
		platformVerifier := "acceptPlatform"
		if v.Op == "attest-cca-reject-platform" {
			platformVerifier = "rejectPlatform"
		}
		evidence := "attest.verifyCcaCollection(" + hexArg(v.Hex) + ", &policy, " + realmVerifier + ", " + platformVerifier + ")"
		if v.Op == "attest-cca-separated" {
			evidence = "attest.verifyCca(&attest.CcaEvidence { realm: " + hexArg(v.Hex) + ", platform: Some(" + hexArg(v.Hex2) + ") }, &policy, " + realmVerifier + ", " + platformVerifier + ")"
		}
		if v.Op == "attest-cca-no-platform" {
			evidence = "attest.verifyCca(&attest.CcaEvidence { realm: " + hexArg(v.Hex) + ", platform: None }, &policy, " + realmVerifier + ", " + platformVerifier + ")"
		}
		emit(fmt.Sprintf(`{
    let mut policy = attest.CcaPolicy.open()
%s%s%s    match %s {
        Ok(claims) -> "ok rim=${hex.encode(claims.initialMeasurement)} rem0=${hex.encode(claims.extensibleMeasurements[0])} mec=${claims.mecPolicy}"
        Err(err) -> "error:${attestCode(err)}"
    }
}`, nonceLine, rimLine, remLine, evidence))
	case "evidence-snp":
		emit(`{
        let policy = attest.TdxPolicy.production()
        match attest.verifyTdxEvidence(attest.TdxEvidence.SnpIdBlock, &policy) {
            Ok(_) -> "ok"
            Err(err) -> "error:${err.code.name()}"
        }
    }`)
	case "idblock":
		emit(fmt.Sprintf(`match snp.parseIdBlock(%s) {
        Ok(block) -> match snp.parseIdAuth(%s) {
            Ok(auth) -> {
                let author = match auth.authorKey {
                    Some(key) -> hex.encode(sha512.sum384(key.bytes()))
                    None -> "none"
                }
                "ok id=${hex.encode(sha512.sum384(auth.idKey.bytes()))} author=${author}"
            }
            Err(err) -> "error:${snpCode(err)}"
        }
        Err(err) -> "error:${snpCode(err)}"
    }`, hexArg(v.Hex), hexArg(v.Hex2)))
	}
}

func parseCvmResults(t *testing.T, out string) map[string]string {
	t.Helper()
	results := map[string]string{}
	if out == "" {
		return results
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		id, value, ok := strings.Cut(line, "\t")
		if !ok || id == "" {
			t.Fatalf("cvm program emitted a malformed line %q", line)
		}
		if _, dup := results[id]; dup {
			t.Fatalf("cvm program emitted %q twice", id)
		}
		results[id] = value
	}
	return results
}

func TestCvmConformance(t *testing.T) {
	corpus := loadCvmCorpus(t)
	source := buildCvmProgram(corpus)

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
		t.Fatalf("load generated cvm program (%s): %v", sourcePath, err)
	}

	var vmOut, vmErr bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: root}, &vmOut, &vmErr, []string{"main/main.kg"})
	if err != nil || code != 0 {
		t.Fatalf("vm run: err %v exit %d\nstderr:\n%s", err, code, vmErr.String())
	}
	vm := parseCvmResults(t, vmOut.String())

	var native map[string]string
	if testCompiler().CCompiler() == nil {
		t.Log("native engine skipped: no C compiler")
	} else {
		exe := filepath.Join(root, "cvm-conformance")
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
		native = parseCvmResults(t, out.String())
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

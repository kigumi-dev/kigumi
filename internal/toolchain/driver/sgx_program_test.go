package driver_test

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const sgxPrelude = `import sgx from std/tee/sgx
import x509 from std/crypto/x509
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

fn joined(items: &Array[String]) -> String {
    let mut out = ""
    let mut i: usize = 0
    for i < items.len() {
        out = if i == 0 { items[i] } else { "${out},${items[i]}" }
        i = i + 1
    }
    out
}

fn number(value: Bytes?) -> String {
    match value {
        Some(bytes) -> hex.encode(bytes)
        None -> "none"
    }
}

fn verified(r: sgx.VerificationResult) -> String {
    "status=${r.status.name()} platform=${r.appraisal.platform.status.name()} qe=${r.appraisal.qe.status.name()} date=${r.appraisal.tcbDate} advisories=${joined(&r.advisoryIds)}"
}

fn root(c: &x509.Certificate) -> String {
    let key = match &c.publicKey {
        x509.PublicKey.EcP256(_) -> "p256"
        x509.PublicKey.Rsa(_) -> "rsa"
    }
    let selfSigned = match c.verifySignedBy(c) {
        Ok(_) -> true
        Err(_) -> false
    }
    let pathLen = match c.basicConstraints.pathLen {
        Some(n) -> "${n}"
        None -> "none"
    }
    let cn = c.subject.commonName || ""
    "ok key=${key} ca=${p(c.basicConstraints.ca)} pathLen=${pathLen} selfSigned=${p(selfSigned)} cn=${cn} notAfter=${c.validity.notAfterMillis()}"
}
`

type sgxItem struct {
	suite int
	v     sgxVector
}

func sgxItems(m *sgxManifest) []sgxItem {
	var items []sgxItem
	for i, suite := range m.Suites {
		for _, v := range suite.Vectors {
			items = append(items, sgxItem{suite: i, v: v})
		}
	}
	return items
}

func buildSgxProgram(m *sgxManifest) string { return sgxProgram(m, sgxItems(m)) }

func sgxBlobFn(suite int, name string) string {
	return fmt.Sprintf("blob%d_%s", suite, strings.ReplaceAll(name, "-", "_"))
}

// Each referenced blob becomes one function, so shared collateral is not repeated per vector.
func sgxProgram(m *sgxManifest, items []sgxItem) string {
	var prog strings.Builder
	prog.WriteString(sgxPrelude)
	used := map[string]string{}
	var body strings.Builder
	for idx, it := range items {
		suite := m.Suites[it.suite]
		blob := func(name string) string {
			fn := sgxBlobFn(it.suite, name)
			used[fn] = suite.Blobs[name]
			return fn + "()"
		}
		fmt.Fprintf(&body, "\n    // %s/%s\n", suite.Primitive, it.v.ID)
		emitSgxVector(&body, it.v, idx, blob)
		fmt.Fprintf(&body, "    print %q\n", suite.Primitive+"/"+it.v.ID+"\t${result"+fmt.Sprint(idx)+"}")
	}
	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&prog, "\nfn %s() -> String {\n    %q\n}\n", name, used[name])
	}
	prog.WriteString("\nfn main() -> Void! {\n")
	prog.WriteString(body.String())
	prog.WriteString("}\n")
	return prog.String()
}

// sgxWeight approximates a vector's VM cost in P-256 verifications.
func sgxWeight(v sgxVector) int {
	switch v.Op {
	case "sgx-verify":
		if strings.HasPrefix(v.Expected, "error:") {
			return 5
		}
		return 9
	case "sgx-collateral-auth":
		return 4
	case "sgx-pck", "sgx-qe", "sgx-pck-path":
		return 2
	case "sgx-signature", "sgx-crl", "sgx-root":
		return 1
	}
	return 0
}

func shardSgxItems(items []sgxItem, n int) [][]sgxItem {
	order := slices.Clone(items)
	sort.SliceStable(order, func(i, j int) bool { return sgxWeight(order[i].v) > sgxWeight(order[j].v) })
	shards := make([][]sgxItem, n)
	load := make([]int, n)
	for _, it := range order {
		lightest := slices.Index(load, slices.Min(load))
		shards[lightest] = append(shards[lightest], it)
		load[lightest] += sgxWeight(it.v) + 1
	}
	return shards
}

// The VM runs one program on one thread, so the P-256-heavy corpus is split into concurrent shards.
func runSgxShardsOnVM(t *testing.T, m *sgxManifest, n int) map[string]string {
	t.Helper()
	shards := shardSgxItems(sgxItems(m), n)
	modules := make([]*driver.Module, len(shards))
	roots := make([]string, len(shards))
	for i, shard := range shards {
		modules[i], roots[i] = loadSgxProgram(t, sgxProgram(m, shard))
	}
	outs := make([]bytes.Buffer, len(shards))
	errs := make([]error, len(shards))
	var wg sync.WaitGroup
	for i := range shards {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var stderr bytes.Buffer
			code, err := driver.RunWith(modules[i], driver.RunOptions{Cwd: roots[i]}, &outs[i], &stderr, []string{"main/main.kg"})
			if err != nil || code != 0 {
				errs[i] = fmt.Errorf("vm shard %d: err %v exit %d\nstderr:\n%s", i, err, code, stderr.String())
			}
		}()
	}
	wg.Wait()
	results := map[string]string{}
	for i := range shards {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		for key, value := range parseSgxResults(t, outs[i].String()) {
			results[key] = value
		}
	}
	return results
}

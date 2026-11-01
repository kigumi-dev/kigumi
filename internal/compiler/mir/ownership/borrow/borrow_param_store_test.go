package borrow

import (
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const siteHolderTypes = `type V['a] = { pub s &'a String }
type Sink['a] = { pub mut items &'a mut Array[V['a]] }
type Pair['a] = { pub mut x Sink['a], pub mut y Sink['a] }
`

// chainSource is f calling h0, each h calling the next twice and the last calling f twice: one component of n+1 functions.
func chainSource(n int) string {
	var b strings.Builder
	b.WriteString(siteHolderTypes)
	b.WriteString("fn f['a](p: &mut Pair['a], n: Int, x: V['a]) { if n > 0 { h0(p, n - 1, x) } else { p.y.items.push(x) } }\n")
	for i := range n {
		next := fmt.Sprintf("h%d", i+1)
		if i == n-1 {
			next = "f"
		}
		fmt.Fprintf(&b, "fn h%d['a](p: &mut Pair['a], n: Int, x: V['a]) { %s(p, n, x); %s(p, n, x) }\n", i, next, next)
	}
	b.WriteString(`fn go() -> String {
    let mut v = "v"
    let u = "u"
    let mut a = Array.of(V { s: &u })
    let mut b = Array.of(V { s: &u })
    let mut p = Pair { x: Sink { items: &mut a }, y: Sink { items: &mut b } }
    f(&mut p, 0, V { s: &v })
    v = "zz"
    "${a[0].s} ${v}"
}
print go()
`)
	return b.String()
}

// rotationSource is one fn with k `&mut` holders that passes them on rotated by one, storing through the first.
func rotationSource(k int) string {
	var params, rot, decls, args []string
	for i := range k {
		params = append(params, fmt.Sprintf("s%d: &mut Sink['a]", i))
		rot = append(rot, fmt.Sprintf("s%d", (i+1)%k))
		decls = append(decls, fmt.Sprintf("    let mut a%d = Array.of(V { s: &u })\n    let mut k%d = Sink { items: &mut a%d }\n", i, i, i))
		args = append(args, fmt.Sprintf("&mut k%d", i))
	}
	return siteHolderTypes + fmt.Sprintf("fn f['a](%s, n: Int, x: V['a]) { if n > 0 { f(%s, n - 1, x) } else { s0.items.push(x) } }\n",
		strings.Join(params, ", "), strings.Join(rot, ", ")) +
		fmt.Sprintf("fn go() -> String {\n    let mut v = \"v\"\n    let u = \"u\"\n%s    f(%s, 3, V { s: &v })\n    \"${v}\"\n}\nprint go()\n",
			strings.Join(decls, ""), strings.Join(args, ", "))
}

func buildSites(t *testing.T, src string, budget time.Duration) (*fixture, *midir.Func) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../../std"), main), IsrContracts: opdef.IsrContracts}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	start := cpuTime()
	p := buildTypedFixture(res)
	took := cpuTime() - start
	t.Logf("built in %v of CPU", took)
	if took > budget {
		t.Errorf("building took %v of CPU, over %v", took, budget)
	}
	for _, f := range p.Funcs {
		for _, d := range f.Diags {
			t.Errorf("%s: unexpected %s", f.Name, d.Msg)
		}
	}
	return p, p.ByEnt[res.PackageMember(res.PackageByPath("main"), "f")]
}

// cpuTime is the process's CPU time so far, which other load on the machine does not inflate.
func cpuTime() time.Duration {
	var u syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &u) != nil {
		return 0
	}
	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}

// The summaries of one call-graph component are solved together, so their cost grows with its size, not its paths.
func TestParamStoreSitesComponentCost(t *testing.T) {
	p, f := buildSites(t, chainSource(22), time.Second)
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: f}
	if sites := c.paramStoreSites(f, 0); len(sites) != 1 || len(sites[0].path) == 0 || sites[0].path[0] != 1 {
		t.Fatalf("chain: sites = %v, want one under p.y", sites)
	}
	t.Logf("chain: %d summary runs", c.siteRuns)
	if c.siteRuns > 8*23 {
		t.Errorf("chain: %d summary runs for 23 functions", c.siteRuns)
	}
	p, f = buildSites(t, rotationSource(20), 2*time.Second)
	c = &borrowCheck{p: p.input, state: New(p.input, nil), f: f}
	for i := range 20 {
		if len(c.paramStoreSites(f, i)) == 0 {
			t.Fatalf("rotation: parameter %d stores nothing", i)
		}
	}
	t.Logf("rotation: %d summary runs", c.siteRuns)
	if c.siteRuns > 2*22*22 {
		t.Errorf("rotation: %d summary runs for 22 parameters", c.siteRuns)
	}
	p, f = buildSites(t, rotationSource(60), 3*time.Second)
	c = &borrowCheck{p: p.input, state: New(p.input, nil), f: f}
	c.paramStoreSites(f, 0)
	t.Logf("rotation of 60: %d summary runs", c.siteRuns)
	if c.siteRuns > 4*62 {
		t.Errorf("rotation of 60: %d summary runs for 62 parameters", c.siteRuns)
	}
}

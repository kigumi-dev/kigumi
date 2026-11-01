package module_test

import (
	"strings"
	"testing"

	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/module"
)

const modHead = "Module {\n    name: \"%s\"\n    kigumi: \"0.1\"\n}\n"

func manifest(name, rest string) []byte {
	return []byte(strings.Replace(modHead, "%s", name, 1) + rest)
}

func TestParseModFile(t *testing.T) {
	src := "Module { name: \"github.com/acme/app\", kigumi: \"0.1\" }\n\nRequire {\n    name: \"github.com/acme/greet\"\n    version: \"v0.1.0\"\n}\nLink { library: \"m\" }\nSource { c: \"bridge/x.c\" }\nHeader { path: \"vendor/x.h\", package: \"x\" }\n"
	mf, ds := module.ParseModFile(token.NewFile("mod.kg", []byte(src)))
	if len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", ds)
	}
	if mf.Name != "github.com/acme/app" || mf.Kigumi != "0.1" || len(mf.Requires) != 1 || mf.Requires[0] != (module.Require{Name: "github.com/acme/greet", Version: "v0.1.0"}) || len(mf.Links) != 1 || mf.Sources[0] != (module.NativeSource{Path: "bridge/x.c"}) || mf.Headers[0] != (module.HeaderImport{Path: "vendor/x.h", Package: "x"}) {
		t.Fatalf("parsed %+v", mf)
	}
	if mf.Zig != "" {
		t.Fatalf("zig is optional and was not given: %+v", mf)
	}
	bad := map[string]string{
		"Module { name: \"a\" }\n":                                                                   "needs `kigumi`",
		"Module { name: \"a\", kigumi: \"9.0\" }\n":                                                  "needs kigumi 9.0",
		"Module { name: \"a\", kigumi: \"not-a-version\" }\n":                                        "is not a number",
		"Require { name: \"x\", version: \"v1.0.0\" }\n":                                             "needs a Module record",
		"Module { name: \"a\", kigumi: \"0.1\" }\nReplace { name: \"x\", path: \"y\" }\n":            "belongs in mod.local.kg",
		"Module { name: \"a\", kigumi: \"0.1\" }\nLink { library: \"/usr/lib/x\" }\n":                "is a name, not a path",
		"Module { name: \"a\", kigumi: \"0.1\" }\nLink { search: \"../elsewhere\" }\n":               "inside the module",
		"Module { name: \"a\", kigumi: \"0.1\" }\nSource { c: \"/abs/x.c\" }\n":                      "stay inside the module",
		"Module { name: \"a\", kigumi: \"0.1\" }\nSource { c: \"x.c\", asm: \"y.s\" }\n":             "exactly one of",
		"Module { name: \"a\", kigumi: \"0.1\" }\nHeader { path: \"x.h\" }\n":                        "needs both",
		"Module { name: \"a\", kigumi: \"0.1\" }\nHeader { path: \"/abs/x.h\", package: \"x\" }\n":   "file inside the module",
		"Module { name: \"a\", kigumi: \"0.1\" }\nHeader { path: \"x.h\", package: \"1x\" }\n":       "look like an identifier",
		"Module { name: \"a/../b\", kigumi: \"0.1\" }\n":                                             "has a `..` segment",
		"Module { name: \"a//b\", kigumi: \"0.1\" }\n":                                               "empty path segment",
		"Module { name: \"a b\", kigumi: \"0.1\" }\n":                                                "may only use letters",
		"Module { name: \"a\", kigumi: \"0.1\" }\nRequire { name: \"x\", version: \"1.0\" }\n":       "is not `v<major>.<minor>.<patch>",
		"Module { name: \"a\", kigumi: \"0.1\" }\nRequire { name: \"x\", version: \"v1.0\" }\n":      "is not `v<major>.<minor>.<patch>",
		"Module { name: \"a\", kigumi: \"0.1\" }\nRequire { name: \"x\", version: \"v1.0.0-01\" }\n": "leading zero",
		"Module { name: \"a\", kigumi: \"0.1\" }\nRequire { name: \"x@y\", version: \"v1.0.0\" }\n":  "may only use letters",
		"Dep { name: \"x\" }\n": "unknown record Dep",
		"Module { name: 3 }\n":  "plain string literal",
		"let x = 1\n":           "expected a record",
	}
	for src, want := range bad {
		_, ds := module.ParseModFile(token.NewFile("mod.kg", []byte(src)))
		found := false
		for _, d := range ds {
			found = found || strings.Contains(d.Msg, want)
		}
		if !found {
			t.Errorf("%q: want %q, got %+v", src, want, ds)
		}
	}
}

func TestParseModFileZig(t *testing.T) {
	onModule := "Module { name: \"a\", kigumi: \"0.1\", zig: \"0.17.0\" }\n"
	mf, ds := module.ParseModFile(token.NewFile("mod.kg", []byte(onModule)))
	if len(ds) != 0 || mf.Zig != "0.17.0" {
		t.Fatalf("zig on Module: mf=%+v ds=%+v", mf, ds)
	}
	standalone := "Module { name: \"a\", kigumi: \"0.1\" }\nZig { version: \"0.17.0\" }\n"
	mf, ds = module.ParseModFile(token.NewFile("mod.kg", []byte(standalone)))
	if len(ds) != 0 || mf.Zig != "0.17.0" {
		t.Fatalf("standalone Zig record: mf=%+v ds=%+v", mf, ds)
	}
	bad := "Module { name: \"a\", kigumi: \"0.1\", zig: \"not-a-version\" }\n"
	_, ds = module.ParseModFile(token.NewFile("mod.kg", []byte(bad)))
	found := false
	for _, d := range ds {
		found = found || strings.Contains(d.Msg, "<major>.<minor>.<patch>")
	}
	if !found {
		t.Errorf("a malformed zig pin should be rejected, got %+v", ds)
	}
}

func TestRepoOf(t *testing.T) {
	cases := []struct{ name, url, wantURL, wantSub string }{
		{"github.com/kigumilang/exp/sgx", "", "https://github.com/kigumilang/exp.git", "sgx"},
		{"github.com/hayao0819/greet", "", "https://github.com/hayao0819/greet.git", ""},
		{"example.com/team/mono.git/tools", "", "https://example.com/team/mono.git", "tools"},
		{"example.com/team/mono.git/tools", "ssh://git@example.com/team/mono.git", "ssh://git@example.com/team/mono.git", "tools"},
		{"github.com/kigumilang/exp/sgx", "/tmp/checkouts/github.com/kigumilang/exp", "/tmp/checkouts/github.com/kigumilang/exp", "sgx"},
		{"greet", "/tmp/greet.git", "/tmp/greet.git", ""},
	}
	for _, c := range cases {
		url, sub, ok := module.RepoOf(module.Require{Name: c.name, URL: c.url})
		if !ok || url != c.wantURL || sub != c.wantSub {
			t.Errorf("%s (%s): got %q %q %v, want %q %q", c.name, c.url, url, sub, ok, c.wantURL, c.wantSub)
		}
	}
	if _, _, ok := module.RepoOf(module.Require{Name: "greet"}); ok {
		t.Error("a short name has no derivable repo")
	}
	if module.TagOf("sgx", "v0.1.0") != "sgx/v0.1.0" || module.TagOf("", "v1.0.0") != "v1.0.0" || !module.IsCommitHash("3f2c9a1") || module.IsCommitHash("v0.1.0") {
		t.Error("tag or hash rules")
	}
	sha256Id := strings.Repeat("a", 64)
	if !module.IsCommitHash(sha256Id) {
		t.Error("a full sha256 object id should read as a commit, not a tag")
	}
}

// TestTagCommit checks the lock's algorithm-tagged commit id and that a
// pre-existing, untagged lock entry still normalizes to it.
func TestTagCommit(t *testing.T) {
	sha1Id := strings.Repeat("a", 40)
	sha256Id := strings.Repeat("b", 64)
	if got := module.TagCommit(sha1Id); got != "sha1:"+sha1Id {
		t.Errorf("TagCommit(sha1) = %q", got)
	}
	if got := module.TagCommit(sha256Id); got != "sha256:"+sha256Id {
		t.Errorf("TagCommit(sha256) = %q", got)
	}
	if got := module.NormalizeCommit(sha1Id); got != module.TagCommit(sha1Id) {
		t.Errorf("NormalizeCommit of a legacy bare hash = %q, want %q", got, module.TagCommit(sha1Id))
	}
	if got := module.NormalizeCommit(module.TagCommit(sha1Id)); got != module.TagCommit(sha1Id) {
		t.Errorf("NormalizeCommit of an already-tagged id must be a no-op, got %q", got)
	}
}

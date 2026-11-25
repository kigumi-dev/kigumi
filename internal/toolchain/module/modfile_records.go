package module

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/version"
)

// modFileOf assembles records into a manifest. module is false for a
// script's leading records, which name no module.
func modFileOf(recs []Record, module bool) (ModFile, []diag.Diagnostic) {
	var mf ModFile
	var ds []diag.Diagnostic
	seenModule := false
	for _, r := range recs {
		switch r.Kind {
		case "Module":
			if seenModule {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "only one Module record is allowed"))
			}
			seenModule = true
			mf.Name, mf.Kigumi = r.Fields["name"], r.Fields["kigumi"]
			if err := ValidModuleName(mf.Name); err != nil && mf.Name != "" {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, err.Error()))
			}
			if mf.Kigumi == "" {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Module needs `kigumi`, the toolchain version the module is written for"))
			}
			if mf.Zig = r.Fields["zig"]; mf.Zig != "" {
				mf.ZigSpan = r.Span
			}
		case "Kigumi":
			mf.Kigumi = r.Fields["version"]
		case "Zig":
			if mf.Zig = r.Fields["version"]; mf.Zig != "" {
				mf.ZigSpan = r.Span
			}
		case "Require":
			versionOK := true
			rawVersion := r.Fields["version"]
			var versionRange *VersionRange
			if rawVersion == "" {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Require needs a version"))
				versionOK = false
			} else if rng, err := ParseVersionRequest(rawVersion); err != nil {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, err.Error()))
				versionOK = false
			} else {
				versionRange = rng
			}
			alias := r.Fields["alias"]
			if alias != "" {
				if err := ValidAlias(alias); err != nil {
					ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, err.Error()))
				}
			}
			if err := ValidModuleName(r.Fields["name"]); err != nil {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, err.Error()))
			} else if versionOK && versionRange == nil {
				if err := CheckVersionForName(r.Fields["name"], rawVersion); err != nil {
					ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, err.Error()))
				}
			}
			mf.Requires = append(mf.Requires, Require{Name: r.Fields["name"], Version: rawVersion, URL: r.Fields["url"], Alias: alias, Range: versionRange})
		case "Replace":
			ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Replace belongs in "+LocalName+", which is never published"))
		case "Link":
			ds = append(ds, linkDiags(r)...)
			mf.Links = append(mf.Links, Link{Library: r.Fields["library"], Search: r.Fields["search"]})
		case "Source":
			c, asm := r.Fields["c"], r.Fields["asm"]
			switch {
			case (c == "") == (asm == ""):
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Source needs exactly one of `c` or `asm`: a file or a directory relative to the module"))
			case !insideModule(c + asm):
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Source paths stay inside the module"))
			}
			mf.Sources = append(mf.Sources, NativeSource{Path: c + asm, Asm: asm != ""})
		case "Interrupt":
			ds = append(ds, mf.addInterrupt(r)...)
		case "Embedder":
			sym := r.Fields["symbol"]
			if !validCSymbol.MatchString(sym) {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, "Embedder needs a `symbol` that is a C identifier (letters, digits, `_`)"))
			}
			mf.Embedder = append(mf.Embedder, sym)
		case "Header":
			ds = append(ds, headerDiags(r)...)
			mf.Headers = append(mf.Headers, HeaderImport{Path: r.Fields["path"], Package: r.Fields["package"]})
		}
	}
	ds = append(ds, requireAliasDiags(mf, recs)...)
	if mf.Kigumi != "" {
		if atLeast, err := versionAtLeast(version.Toolchain, mf.Kigumi); err != nil {
			ds = append(ds, diag.Errorf(diag.Location{Span: recs[0].Span}, err.Error()))
		} else if !atLeast {
			ds = append(ds, diag.Errorf(diag.Location{Span: recs[0].Span}, fmt.Sprintf("this module needs kigumi %s; this toolchain is %s", mf.Kigumi, version.Toolchain)))
		}
	}
	if mf.Zig != "" {
		if err := ValidZigVersion(mf.Zig); err != nil {
			ds = append(ds, diag.Errorf(diag.Location{Span: mf.ZigSpan}, err.Error()))
		}
	}
	if module && !seenModule {
		span := token.Span{}
		if len(recs) > 0 {
			span = recs[0].Span
		}
		ds = append(ds, diag.Errorf(diag.Location{Span: span}, "the manifest needs a Module record"))
	}
	return mf, ds
}

// requireAliasDiags checks that each alias is a unique import prefix: no two
// Requires may share an effective key or overlapping alias prefixes, and an
// alias may not overlap the root module's own package prefix.
func requireAliasDiags(mf ModFile, recs []Record) []diag.Diagnostic {
	var ds []diag.Diagnostic
	keyAt := map[string]Record{}
	aliasAt := map[string]Record{}
	names := map[string]bool{}
	for _, r := range recs {
		if r.Kind != "Require" {
			continue
		}
		name, alias := r.Fields["name"], r.Fields["alias"]
		if alias == "" {
			// A real name may only collide with an alias; duplicate plain
			// Requires keep the old tolerant MVS behavior.
			if prev, dup := keyAt[name]; dup && prev.Fields["alias"] != "" {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, fmt.Sprintf("alias %q collides with the module %q required here", prev.Fields["alias"], name)))
			}
			names[name] = true
			continue
		}
		if names[alias] {
			ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, fmt.Sprintf("alias %q collides with the module %q required without an alias", alias, alias)))
		}
		if _, dup := keyAt[alias]; dup {
			ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, fmt.Sprintf("alias %q is used by two Require records", alias)))
			continue
		}
		for prevAlias := range aliasAt {
			if prefixOverlap(alias, prevAlias) {
				ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, fmt.Sprintf("alias %q overlaps alias %q", alias, prevAlias)))
			}
		}
		if mf.Name != "" && prefixOverlap(alias, mf.Name) {
			ds = append(ds, diag.Errorf(diag.Location{Span: r.Span}, fmt.Sprintf("alias %q overlaps this module's name %q", alias, mf.Name)))
		}
		keyAt[alias] = r
		aliasAt[alias] = r
	}
	return ds
}

// prefixOverlap reports whether one path is the other or contains the other
// as a leading segment path, the relation that makes two import prefixes
// claim the same package paths.
func prefixOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

// linkDiags keeps Link records to what cannot reach outside the module: a
// bare library name and a relative search directory.
func linkDiags(r Record) []diag.Diagnostic {
	lib, search := r.Fields["library"], r.Fields["search"]
	switch {
	case lib == "" && search == "":
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, "Link needs a library or a search path")}
	case strings.ContainsAny(lib, `/\`):
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, "Link library is a name, not a path")}
	case search != "" && !insideModule(search):
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, "Link search must be a directory inside the module")}
	}
	return nil
}

var validCSymbol = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var validPackageName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// headerDiags keeps a Header record to a single header file inside the
// module, imported under a plain package-name-shaped identifier.
func headerDiags(r Record) []diag.Diagnostic {
	path, pkg := r.Fields["path"], r.Fields["package"]
	switch {
	case path == "" || pkg == "":
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, "Header needs both `path` and `package`")}
	case !insideModule(path):
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, "Header path must be a file inside the module")}
	case !validPackageName.MatchString(pkg):
		return []diag.Diagnostic{diag.Errorf(diag.Location{Span: r.Span}, "Header package must look like an identifier (letters, digits, `_`)")}
	}
	return nil
}

// insideModule accepts relative paths that never leave the module.
func insideModule(p string) bool {
	if p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, `\`) {
		return false
	}
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

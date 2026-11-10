package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
)

// versionFile is the single source of the release, ABI and toolchain versions.
// The release workflow refuses to publish a tag that disagrees with it.
const versionFile = "internal/toolchain/version/version.go"

// VersionInfo is the subset of the toolchain version constants a release
// records in its manifest and SBOM.
type VersionInfo struct {
	Release             string `json:"release"`
	Tag                 string `json:"tag"`
	Toolchain           string `json:"toolchain"`
	Zig                 string `json:"zig"`
	ABIRevision         int    `json:"abiRevision"`
	CanonicalABIVersion string `json:"canonicalAbiVersion"`
}

// readVersion extracts the constants from version.go. It parses the file
// instead of importing the parent module so the release tool stays a small,
// dependency-free module that can run before the toolchain is built.
func readVersion(root string) (VersionInfo, error) {
	path := filepath.Join(root, versionFile)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("parse %s: %w", path, err)
	}
	strs := map[string]string{}
	ints := map[string]int{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range vs.Names {
				if i >= len(vs.Values) {
					continue
				}
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok {
					continue
				}
				switch lit.Kind {
				case token.STRING:
					v, err := strconv.Unquote(lit.Value)
					if err != nil {
						return VersionInfo{}, fmt.Errorf("%s: %s: %w", path, name.Name, err)
					}
					strs[name.Name] = v
				case token.INT:
					v, err := strconv.Atoi(lit.Value)
					if err != nil {
						return VersionInfo{}, fmt.Errorf("%s: %s: %w", path, name.Name, err)
					}
					ints[name.Name] = v
				}
			}
		}
	}
	required := func(name string) (string, error) {
		v, ok := strs[name]
		if !ok || v == "" {
			return "", fmt.Errorf("%s: missing string constant %s", path, name)
		}
		return v, nil
	}
	var v VersionInfo
	if v.Release, err = required("Release"); err != nil {
		return VersionInfo{}, err
	}
	if v.Toolchain, err = required("Toolchain"); err != nil {
		return VersionInfo{}, err
	}
	if v.Zig, err = required("Zig"); err != nil {
		return VersionInfo{}, err
	}
	if v.CanonicalABIVersion, err = required("CanonicalABIVersion"); err != nil {
		return VersionInfo{}, err
	}
	v.ABIRevision = ints["AbiRevision"]
	if v.ABIRevision <= 0 {
		return VersionInfo{}, fmt.Errorf("%s: missing positive integer constant AbiRevision", path)
	}
	v.Tag = "v" + v.Release
	return v, nil
}

func runVersion(args []string) error {
	fs := newFlagSet("version")
	root := fs.String("root", ".", "repository root")
	field := fs.String("field", "", "print one field (release, tag, toolchain, zig, abi, canonical-abi) instead of the whole record")
	expect := fs.String("expect", "", "fail unless this tag or release matches the version file")
	asJSON := fs.Bool("json", false, "print the version record as JSON")
	if err := parse(fs, args); err != nil {
		return err
	}
	v, err := readVersion(*root)
	if err != nil {
		return err
	}
	if *expect != "" {
		want := strings.TrimPrefix(*expect, "v")
		if want != v.Release {
			return fmt.Errorf("tag %q does not match version.go Release %q", *expect, v.Release)
		}
	}
	if *field != "" {
		var out string
		switch *field {
		case "release":
			out = v.Release
		case "tag":
			out = v.Tag
		case "toolchain":
			out = v.Toolchain
		case "zig":
			out = v.Zig
		case "abi":
			out = strconv.Itoa(v.ABIRevision)
		case "canonical-abi":
			out = v.CanonicalABIVersion
		default:
			return fmt.Errorf("unknown field %q", *field)
		}
		fmt.Println(out)
		return nil
	}
	if *asJSON {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("release=%s\ntag=%s\ntoolchain=%s\nzig=%s\nabi=%d\ncanonical-abi=%s\n",
		v.Release, v.Tag, v.Toolchain, v.Zig, v.ABIRevision, v.CanonicalABIVersion)
	return nil
}

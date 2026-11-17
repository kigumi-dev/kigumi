package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type vector struct {
	ID            string         `json:"id"`
	Op            string         `json:"op"`
	Quote         string         `json:"quote,omitempty"`
	Hex           string         `json:"hex,omitempty"`
	Hex2          string         `json:"hex2,omitempty"`
	Anchors       string         `json:"anchors,omitempty"`
	Now           int64          `json:"now,omitempty"`
	ReportData    string         `json:"reportData,omitempty"`
	Measurement   string         `json:"measurement,omitempty"`
	CpuSvn        string         `json:"cpuSvn,omitempty"`
	PceSvn        uint16         `json:"pceSvn,omitempty"`
	Args          []string       `json:"args,omitempty"`
	Collateral    *collateralRef `json:"collateral,omitempty"`
	Accept        string         `json:"accept,omitempty"`
	MinEvaluation uint32         `json:"minEvaluation,omitempty"`
	Expected      string         `json:"expected"`
}

type suite struct {
	Primitive string            `json:"primitive"`
	Source    string            `json:"source"`
	Blobs     map[string]string `json:"blobs,omitempty"`
	Vectors   []vector          `json:"vectors"`
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func decode(value string) []byte { return must(hex.DecodeString(value)) }

func main() {
	check := flag.Bool("check", false, "check that the generated fixtures are current")
	dir := flag.String("dir", "testdata/conformance/sgx", "fixture directory")
	flag.Parse()
	paths := must(filepath.Glob(filepath.Join(*dir, "sgx_*.json")))
	var all []suite
	for _, path := range paths {
		var file suite
		if err := json.Unmarshal(must(os.ReadFile(path)), &file); err != nil {
			panic(err)
		}
		all = append(all, file)
	}
	var base []byte
	for _, file := range all {
		if file.Primitive == "sgx-qe" {
			base = decode(file.Vectors[0].Hex)
		}
	}
	if len(base) < 1020 {
		panic("missing base quote")
	}
	material := makeMaterial()
	for i, path := range paths {
		file := &all[i]
		for j := range file.Vectors {
			regenerate(&file.Vectors[j], material, base)
		}
		write(path, *file, *check)
	}
	write(filepath.Join(*dir, "pck_paths.json"), pathVectors(material), *check)
	collateral := newWorld()
	write(filepath.Join(*dir, "collateral_verify.json"), verifySuite(collateral), *check)
	write(filepath.Join(*dir, "collateral_parts.json"), partsSuite(collateral), *check)
}

func write(path string, file suite, check bool) {
	data := append(must(json.MarshalIndent(file, "", "  ")), '\n')
	if check {
		if !bytes.Equal(data, must(os.ReadFile(path))) {
			panic(fmt.Sprintf("stale fixture %s", path))
		}
	} else if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
}

package driver

import (
	"debug/elf"
	"debug/pe"
	"fmt"
	"os"
	"strings"
)

// objSymbol is one symbol of a compiled object, enough to audit a
// freestanding archive's contract with its embedder.
type objSymbol struct {
	Name    string
	Defined bool
	TLS     bool
}

// readObjectSymbols reads an ELF (freestanding), PE/COFF (UEFI) or wasm object's
// symbol table; debug/pe also parses a bare .o with no DOS/PE header.
func readObjectSymbols(path string) ([]objSymbol, error) {
	if data, err := os.ReadFile(path); err == nil && isWasmObject(data) {
		return wasmObjSymbols(data)
	}
	if f, err := elf.Open(path); err == nil {
		defer f.Close()
		return elfObjSymbols(f)
	}
	if f, err := pe.Open(path); err == nil {
		defer f.Close()
		return peObjSymbols(f)
	}
	return nil, fmt.Errorf("%s: not a recognized object format", path)
}

func elfObjSymbols(f *elf.File) ([]objSymbol, error) {
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	out := make([]objSymbol, 0, len(syms))
	for _, s := range syms {
		if s.Name == "" {
			continue
		}
		out = append(out, objSymbol{
			Name:    s.Name,
			Defined: s.Section != elf.SHN_UNDEF,
			TLS:     elf.ST_TYPE(s.Info) == elf.STT_TLS,
		})
	}
	return out, nil
}

// peObjSymbols treats section number 0 (IMAGE_SYM_UNDEFINED) as undefined. COFF has no
// per-symbol TLS type like ELF's STT_TLS (32-bit x86 COFF prefixes every C name with an underscore, dropped here); a symbol is TLS if its section is named .tls or .tls$*.
func peObjSymbols(f *pe.File) ([]objSymbol, error) {
	out := make([]objSymbol, 0, len(f.Symbols))
	decorated := f.Machine == pe.IMAGE_FILE_MACHINE_I386
	for _, s := range f.Symbols {
		if s.Name == "" {
			continue
		}
		if decorated {
			s.Name = strings.TrimPrefix(s.Name, "_")
		}
		tls := false
		if s.SectionNumber > 0 && int(s.SectionNumber) <= len(f.Sections) {
			tls = strings.HasPrefix(f.Sections[s.SectionNumber-1].Name, ".tls")
		}
		out = append(out, objSymbol{Name: s.Name, Defined: s.SectionNumber != 0, TLS: tls})
	}
	return out, nil
}

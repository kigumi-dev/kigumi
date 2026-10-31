package object

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	wasmSymUndefined    = 0x10
	wasmSymExplicitName = 0x40
	wasmSymTLS          = 0x100
	wasmKindData        = 1
	wasmKindSection     = 3
)

var errWasmTruncated = errors.New("truncated wasm object")

type wasmReader struct {
	b   []byte
	err error
}

func (r *wasmReader) uvar() uint64 {
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.err = errWasmTruncated
		r.b = nil
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *wasmReader) take(n uint64) []byte {
	if n > uint64(len(r.b)) {
		r.err = errWasmTruncated
		r.b = nil
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *wasmReader) byte() byte {
	if b := r.take(1); len(b) == 1 {
		return b[0]
	}
	return 0
}

func (r *wasmReader) name() string { return string(r.take(r.uvar())) }

func isWasmObject(data []byte) bool {
	return len(data) >= 8 && string(data[:4]) == "\x00asm"
}

// wasmObjSymbols reads the linking section's symbol table; an undefined
// symbol has no name of its own and is named by its entry in the import section.
func wasmObjSymbols(data []byte) ([]Symbol, error) {
	r := &wasmReader{b: data[8:]}
	var imports [5][]string
	var linking []byte
	for len(r.b) > 0 && r.err == nil {
		id := r.byte()
		body := &wasmReader{b: r.take(r.uvar())}
		switch id {
		case 0:
			if body.name() == "linking" {
				linking = body.b
			}
		case 2:
			readWasmImports(body, &imports)
		}
		if body.err != nil && id != 0 {
			return nil, body.err
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	if linking == nil {
		return nil, errors.New("wasm object has no linking section")
	}
	return readWasmSymtab(&wasmReader{b: linking}, imports)
}

func readWasmImports(r *wasmReader, imports *[5][]string) {
	for n := r.uvar(); n > 0 && r.err == nil; n-- {
		r.name()
		field := r.name()
		kind := r.byte()
		switch kind {
		case 0:
			r.uvar()
		case 1:
			r.byte()
			readWasmLimits(r)
		case 2:
			readWasmLimits(r)
		case 3:
			r.byte()
			r.byte()
		case 4:
			r.byte()
			r.uvar()
		default:
			r.err = fmt.Errorf("unknown wasm import kind %d", kind)
		}
		if kind != 2 && kind < 5 {
			imports[kind] = append(imports[kind], field)
		}
	}
}

func readWasmLimits(r *wasmReader) {
	flags := r.uvar()
	r.uvar()
	if flags&1 != 0 {
		r.uvar()
	}
}

func readWasmSymtab(r *wasmReader, imports [5][]string) ([]Symbol, error) {
	r.uvar()
	var out []Symbol
	for len(r.b) > 0 && r.err == nil {
		kind := r.byte()
		sub := &wasmReader{b: r.take(r.uvar())}
		if kind != 8 {
			continue
		}
		for n := sub.uvar(); n > 0 && sub.err == nil; n-- {
			kind, flags := sub.byte(), sub.uvar()
			undefined := flags&wasmSymUndefined != 0
			var name string
			switch kind {
			case wasmKindData:
				name = sub.name()
				if !undefined {
					sub.uvar()
					sub.uvar()
					sub.uvar()
				}
			case wasmKindSection:
				sub.uvar()
				continue
			default:
				idx := sub.uvar()
				if !undefined || flags&wasmSymExplicitName != 0 {
					name = sub.name()
				} else if list := imports[wasmImportSpace(kind)]; idx < uint64(len(list)) {
					name = list[idx]
				}
			}
			if name != "" {
				out = append(out, Symbol{Name: name, Defined: !undefined, TLS: flags&wasmSymTLS != 0})
			}
		}
		if sub.err != nil {
			return nil, sub.err
		}
	}
	return out, r.err
}

// wasmImportSpace maps a symbol kind (function, data, global, section, event, table) to the import kind of the same index space.
func wasmImportSpace(symKind byte) int {
	switch symKind {
	case 0:
		return 0
	case 2:
		return 3
	case 4:
		return 4
	case 5:
		return 1
	}
	return 0
}

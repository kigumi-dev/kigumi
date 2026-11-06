// Package object rewrites linkable objects; this file drops COFF debug sections that carry a
// temp path in S_OBJNAME.
package object

import (
	"encoding/binary"
	"fmt"
	"os"
)

// StripCOFFDebug removes the contents of every .debug section from a COFF object while leaving
// its section header in place. COFF symbol indices and section numbers stay valid, so callers can
// rewrite an object in place without renumbering symbols.
func StripCOFFDebug(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 20 {
		return fmt.Errorf("coff: file is too short")
	}
	nsec := int(binary.LittleEndian.Uint16(data[2:4]))
	hdr := 20 + nsec*40
	if hdr > len(data) {
		return fmt.Errorf("coff: section headers are truncated")
	}
	debug := func(name string) bool {
		return len(name) >= 6 && name[:6] == ".debug"
	}
	aligned := func(out []byte) []byte {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		return out
	}
	u32 := func(off int) uint32 { return binary.LittleEndian.Uint32(data[off : off+4]) }
	put := func(out []byte, off int, v uint32) { binary.LittleEndian.PutUint32(out[off:off+4], v) }
	copyRange := func(out []byte, ptr, size uint32) ([]byte, error) {
		start, end := int(ptr), int(ptr)+int(size)
		if start < 0 || end < start || end > len(data) {
			return out, fmt.Errorf("coff: section range %d+%d is outside the file", start, size)
		}
		return append(out, data[start:end]...), nil
	}

	out := append([]byte{}, data[:hdr]...)
	put(out, 4, 0) // TimeDateStamp
	debugSec := make([]bool, nsec)
	// Section headers are copied as-is, then each kept section's data is appended compactly.
	for i := 0; i < nsec; i++ {
		h := 20 + i*40
		name := string(data[h : h+8])
		if j := len(name); j > 0 {
			for k, c := range name {
				if c == 0 {
					name = name[:k]
					break
				}
			}
		}
		if debug(name) {
			debugSec[i] = true
			put(out, h+16, 0) // SizeOfRawData
			put(out, h+20, 0) // PointerToRawData
			put(out, h+24, 0) // PointerToRelocations
			put(out, h+32, 0) // NumberOfRelocations (u16 + u16)
			continue
		}
		rawSize, rawPtr := u32(h+16), u32(h+20)
		if rawSize == 0 {
			put(out, h+20, 0)
		}
		if rawSize > 0 && rawPtr != 0 {
			out = aligned(out)
			put(out, h+20, uint32(len(out)))
			out, err = copyRange(out, rawPtr, rawSize)
			if err != nil {
				return err
			}
		}
		relPtr, numRel := u32(h+24), binary.LittleEndian.Uint16(data[h+32:h+34])
		if numRel == 0 {
			put(out, h+24, 0)
		}
		if numRel > 0 && relPtr != 0 {
			out = aligned(out)
			put(out, h+24, uint32(len(out)))
			out, err = copyRange(out, relPtr, uint32(numRel)*10)
			if err != nil {
				return err
			}
		}
		linePtr, numLine := u32(h+28), binary.LittleEndian.Uint16(data[h+34:h+36])
		if numLine == 0 {
			put(out, h+28, 0)
		}
		if numLine > 0 && linePtr != 0 {
			out = aligned(out)
			put(out, h+28, uint32(len(out)))
			out, err = copyRange(out, linePtr, uint32(numLine)*6)
			if err != nil {
				return err
			}
		}
	}

	symPtr := u32(8)
	symCount := u32(12)
	if symPtr != 0 && symCount != 0 {
		symEnd := int(symPtr) + int(symCount)*18
		if symEnd > len(data) {
			return fmt.Errorf("coff: symbol table is truncated")
		}
		end := symEnd
		if symEnd+4 <= len(data) {
			strSize := int(u32(symEnd))
			if strSize >= 4 {
				end = symEnd + strSize
			}
		}
		if end > len(data) {
			return fmt.Errorf("coff: string table is truncated")
		}
		out = aligned(out)
		symOut := len(out)
		put(out, 8, uint32(symOut))
		out = append(out, data[symPtr:end]...)
		// Section symbols carry a checksum of the raw bytes we removed; zero it so the
		// stripped object no longer depends on the debug section's old contents.
		for i := 0; i < int(symCount); {
			off := symOut + i*18
			if off+18 > len(out) {
				break
			}
			aux := int(out[off+17])
			sec := int(int16(binary.LittleEndian.Uint16(out[off+12 : off+14])))
			if out[off+16] == 3 && aux > 0 && sec > 0 && sec <= nsec && debugSec[sec-1] {
				a := off + 18
				if a+12 <= len(out) {
					copy(out[a:a+12], make([]byte, 12))
				}
			}
			i += 1 + aux
		}
	} else {
		put(out, 8, 0)
		put(out, 12, 0)
	}
	return os.WriteFile(path, out, 0o644)
}

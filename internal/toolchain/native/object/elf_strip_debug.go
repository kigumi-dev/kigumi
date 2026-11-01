package object

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

type elfLayout struct {
	order binary.ByteOrder
	is64  bool
	shoff int
	shent int
	shnum int
	shstr int
	phoff int
	phent int
	phnum int
}

type elfSection struct {
	raw    []byte
	name   string
	typ    uint32
	flags  uint64
	offset uint64
	size   uint64
	align  uint64
	link   uint32
}

const (
	shtNobits = 8
	shfAlloc  = 2
)

func (l *elfLayout) word(b []byte, off32, off64 int) uint64 {
	if l.is64 {
		return l.order.Uint64(b[off64:])
	}
	return uint64(l.order.Uint32(b[off32:]))
}

func (l *elfLayout) linkOff() int {
	if l.is64 {
		return 40
	}
	return 24
}

func (l *elfLayout) putWord(b []byte, off32, off64 int, v uint64) {
	if l.is64 {
		l.order.PutUint64(b[off64:], v)
	} else {
		l.order.PutUint32(b[off32:], uint32(v))
	}
}

func parseELFLayout(img []byte) (*elfLayout, error) {
	if len(img) < 64 || !bytes.HasPrefix(img, []byte("\x7fELF")) {
		return nil, fmt.Errorf("not an ELF file")
	}
	l := &elfLayout{is64: img[4] == 2, order: binary.ByteOrder(binary.LittleEndian)}
	if img[5] == 2 {
		l.order = binary.BigEndian
	}
	if l.is64 {
		l.phoff, l.shoff = int(l.order.Uint64(img[32:])), int(l.order.Uint64(img[40:]))
		l.phent, l.phnum = int(l.order.Uint16(img[54:])), int(l.order.Uint16(img[56:]))
		l.shent, l.shnum, l.shstr = int(l.order.Uint16(img[58:])), int(l.order.Uint16(img[60:])), int(l.order.Uint16(img[62:]))
	} else {
		l.phoff, l.shoff = int(l.order.Uint32(img[28:])), int(l.order.Uint32(img[32:]))
		l.phent, l.phnum = int(l.order.Uint16(img[42:])), int(l.order.Uint16(img[44:]))
		l.shent, l.shnum, l.shstr = int(l.order.Uint16(img[46:])), int(l.order.Uint16(img[48:])), int(l.order.Uint16(img[50:]))
	}
	if l.shoff == 0 || l.shnum == 0 || l.shoff+l.shnum*l.shent > len(img) {
		return nil, fmt.Errorf("ELF section table is missing or truncated")
	}
	return l, nil
}

func (l *elfLayout) sections(img []byte) []elfSection {
	secs := make([]elfSection, l.shnum)
	for i := range secs {
		b := img[l.shoff+i*l.shent : l.shoff+(i+1)*l.shent]
		s := &secs[i]
		s.raw = append([]byte{}, b...)
		s.typ = l.order.Uint32(b[4:])
		s.flags = l.word(b, 8, 8)
		s.offset = l.word(b, 16, 24)
		s.size = l.word(b, 20, 32)
		s.link = l.order.Uint32(b[l.linkOff():])
		s.align = l.word(b, 32, 48)
	}
	names := secs[l.shstr]
	for i := range secs {
		off := int(names.offset) + int(l.order.Uint32(secs[i].raw))
		if end := bytes.IndexByte(img[off:], 0); end >= 0 {
			secs[i].name = string(img[off : off+end])
		}
	}
	return secs
}

func isDebugSection(name string) bool {
	return strings.HasPrefix(name, ".debug_") || strings.HasPrefix(name, ".zdebug_") || name == ".gnu_debuglink"
}

// StripDebug removes the DWARF sections of an ELF file and keeps the symbol table.
// Kept sections stay in place when something loaded sits behind them; the rest are packed down.
func StripDebug(path string) error {
	img, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := stripELFDebugImage(img)
	if err != nil || out == nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, info.Mode().Perm())
}

func stripELFDebugImage(img []byte) ([]byte, error) {
	l, err := parseELFLayout(img)
	if err != nil {
		return nil, err
	}
	secs := l.sections(img)
	keep := make([]int, 0, len(secs))
	remap := map[uint32]uint32{}
	for i, s := range secs {
		if i != 0 && isDebugSection(s.name) {
			continue
		}
		remap[uint32(i)] = uint32(len(keep))
		keep = append(keep, i)
	}
	if len(keep) == len(secs) {
		return nil, nil
	}

	fixed := uint64(l.phoff + l.phnum*l.phent)
	for i := 0; i < l.phnum; i++ {
		p := img[l.phoff+i*l.phent:]
		var off, size uint64
		if l.is64 {
			off, size = l.order.Uint64(p[8:]), l.order.Uint64(p[32:])
		} else {
			off, size = uint64(l.order.Uint32(p[4:])), uint64(l.order.Uint32(p[16:]))
		}
		fixed = max(fixed, off+size)
	}
	for _, i := range keep {
		if s := secs[i]; s.flags&shfAlloc != 0 && s.typ != shtNobits {
			fixed = max(fixed, s.offset+s.size)
		}
	}

	out := append([]byte{}, img[:min(uint64(len(img)), fixed)]...)
	for _, i := range keep {
		s := &secs[i]
		if s.typ == shtNobits || s.flags&shfAlloc != 0 || s.offset < fixed || i == 0 {
			continue
		}
		if s.align > 1 {
			for uint64(len(out))%s.align != 0 {
				out = append(out, 0)
			}
		}
		l.putWord(s.raw, 16, 24, uint64(len(out)))
		out = append(out, img[s.offset:s.offset+s.size]...)
	}
	for len(out)%8 != 0 {
		out = append(out, 0)
	}
	shoff := len(out)
	for _, i := range keep {
		s := secs[i]
		l.order.PutUint32(s.raw[l.linkOff():], remap[s.link])
		out = append(out, s.raw...)
	}

	if l.is64 {
		l.order.PutUint64(out[40:], uint64(shoff))
		l.order.PutUint16(out[60:], uint16(len(keep)))
		l.order.PutUint16(out[62:], uint16(remap[uint32(l.shstr)]))
	} else {
		l.order.PutUint32(out[32:], uint32(shoff))
		l.order.PutUint16(out[48:], uint16(len(keep)))
		l.order.PutUint16(out[50:], uint16(remap[uint32(l.shstr)]))
	}
	return out, nil
}

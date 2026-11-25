package vm

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"

	"kigumi/internal/backend/vm/host/shell"
	"kigumi/internal/compiler/sem"
)

// readChunk caps one File.read, like the hosted runtime.
const readChunk = 65536

// fileWriteBufSize is the 4 KiB stdio buffer the hosted runtime's File gets on
// a device such as /dev/full: a smaller write reports success into the buffer
// and the device error surfaces at close/flush, while a write of at least this
// size goes straight to the file, as fwrite does for an empty buffer.
const fileWriteBufSize = 4096

// fileHandle is an open file plus the pending write buffer. w is nil for a
// handle opened read-only, so a write fails immediately like fwrite on a
// read-only stdio stream.
type fileHandle struct {
	f *os.File
	w *bufio.Writer
}

// writableFile wraps an open file the way the hosted runtime's fopen("w")
// stream buffers it.
func writableFile(f *os.File) *fileHandle {
	return &fileHandle{f: f, w: bufio.NewWriterSize(f, fileWriteBufSize)}
}

// write mirrors fwrite: an empty buffer and a full-buffer write bypass the
// buffer, everything else is buffered and the error waits for flush.
func (h *fileHandle) write(p []byte) (int, error) {
	if h.w.Buffered() == 0 && len(p) >= fileWriteBufSize {
		return h.f.Write(p)
	}
	n, err := h.w.Write(p)
	if err != nil {
		// fwrite drops the stream's failed buffer, so a later close/fflush
		// reports success instead of replaying this error.
		h.w.Reset(h.f)
	}
	return n, err
}

// flush pushes buffered writes to the file; drop ignores the error like
// fclose does, close reports it.
func (h *fileHandle) flush() error {
	if h.w == nil {
		return nil
	}
	return h.w.Flush()
}

// cError is the VM fast path's CError result, matching the Kigumi std/fs body
// that fails with ffi.lastError() on the same OS error.
func (m *Machine) cError(err error) *obj {
	var errno syscall.Errno
	code := int64(0)
	if errors.As(err, &errno) {
		code = int64(errno)
	}
	return m.cErrorCode(code, shell.OSErrText(err))
}

func (m *Machine) cErrorMsg(msg string) *obj { return m.cErrorCode(0, msg) }

func (m *Machine) cErrorCode(code int64, text string) *obj {
	ent := m.lookup("std/ffi", "CError")
	ce := mkRecord(ent, []*obj{mkInt(code, 32), mkStr([]byte(text))})
	box := newObj(kBox)
	box.i, box.inner = int64(m.r.Types.Named(ent, nil)), ce
	m.chargeMemory(box)
	return mkVariant(m.variantOf(m.r.Types.ResultEnt(), "Err"), []*obj{box})
}

// pathRaw is the exact OS name bytes of a fs.Path: field 1 (`raw`), not the
// lossy display text in field 0.
func pathRaw(v *obj) []byte { return deref(deref(v).fields[1]).str() }

func pathText(v *obj) string { return string(pathRaw(v)) }

func fileOf(v *obj) *fileHandle {
	fh, _ := deref(deref(v).fields[0]).opaqueData().(*fileHandle)
	return fh
}

// resultPayload is the type inside the callee's `T!` result.
func (m *Machine) resultPayload(fn sem.EntityID) sem.TypeID {
	t, _, _ := m.r.Types.IsResult(m.r.Types.Node(m.r.Fn(fn).Sig).Elem)
	return t
}

func readFile(m *Machine, fn sem.EntityID, a []*obj) *obj {
	fh := fileOf(a[0])
	if fh == nil {
		return m.cErrorMsg("file is closed")
	}
	want := deref(a[2]).i
	if want < 0 || want > readChunk {
		want = readChunk
	}
	if want == 0 {
		return m.ok(mkInt(0, m.numKind(sem.TyUsize)))
	}
	buf := make([]byte, want)
	got, err := fh.f.Read(buf)
	if err != nil && err != io.EOF {
		return m.cError(err)
	}
	arr := deref(a[1])
	for _, b := range buf[:got] {
		arr.fields = append(arr.fields, mkInt(int64(b), 8))
	}
	return m.ok(mkInt(int64(got), m.numKind(sem.TyUsize)))
}

func (m *Machine) fileRecord(fn sem.EntityID, fh *fileHandle) *obj {
	file := m.r.Types.Node(m.resultPayload(fn)).Ent
	return m.ok(mkRecord(file, []*obj{mkOpaque("Ptr", fh)}))
}

func (m *Machine) registerFiles() {
	h := m.host
	m.registerFileReads()
	h["fs.Files.open"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fh, err := os.Open(m.resolvePath(pathText(a[1])))
		if err != nil {
			return m.cError(err)
		}
		return m.fileRecord(fn, &fileHandle{f: fh})
	}
	h["fs.File.read"] = readFile
	h["fs.Files.create"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fh, err := os.OpenFile(m.resolvePath(pathText(a[1])), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			return m.cError(err)
		}
		return m.fileRecord(fn, writableFile(fh))
	}
	h["fs.Files.append"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fh, err := os.OpenFile(m.resolvePath(pathText(a[1])), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return m.cError(err)
		}
		return m.fileRecord(fn, writableFile(fh))
	}
	h["fs.File.write"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fh := fileOf(a[0])
		if fh == nil {
			return m.cErrorMsg("file is closed")
		}
		if fh.w == nil {
			return m.cError(syscall.EBADF)
		}
		n, err := fh.write(deref(a[1]).str())
		if err != nil {
			return m.cError(err)
		}
		return m.ok(mkInt(int64(n), m.numKind(sem.TyUsize)))
	}
	h["fs.File.close"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if fh := fileOf(a[0]); fh != nil {
			if err := fh.flush(); err != nil {
				return m.cError(err)
			}
		}
		return m.ok(unitObj)
	}
	h["fs.File.drop"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if fh := fileOf(a[0]); fh != nil {
			_ = fh.flush()
			fh.f.Close()
		}
		return unitObj
	}
	h["fs.Files.mkdir"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if err := m.mkdirAll(pathText(a[1])); err != nil {
			return m.cError(err)
		}
		return m.ok(unitObj)
	}
	h["fs.Files.exists"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if pathText(a[1]) == "" {
			return mkBool(false)
		}
		_, err := os.Stat(m.resolvePath(pathText(a[1])))
		return mkBool(err == nil)
	}
	h["fs.Files.list"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		dir := pathText(a[1])
		entries, err := os.ReadDir(m.resolvePath(dir))
		if err != nil {
			return m.cError(err)
		}
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		sort.Strings(names)
		if dir != "" && !strings.HasSuffix(dir, "/") {
			dir += "/"
		}
		path := m.r.Types.Node(m.r.Types.Node(m.resultPayload(fn)).Args[0]).Ent
		out := mkArray(nil)
		for _, name := range names {
			raw := []byte(dir + name)
			out.fields = append(out.fields, mkRecord(path, []*obj{
				mkStr([]byte(utf8Lossy(string(raw)))),
				mkBytes(raw),
			}))
		}
		return m.ok(out)
	}
}

// utf8Lossy replaces invalid UTF-8 bytes with U+FFFD like range does,
// keeping the String invariant for dirent names. Native
// mirrors this in ffi.stringLossy.
func utf8Lossy(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteRune(r)
	}
	return b.String()
}

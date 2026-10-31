package vm

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"unicode/utf8"

	"kigumi/internal/backend/vm/host/shell"
	"kigumi/internal/compiler/sem"
)

// defaultReadLimit mirrors std/fs.defaultReadLimit.
const defaultReadLimit = 268435456

func (m *Machine) readLimited(path string, limit int64) ([]byte, string) {
	f, err := os.Open(m.resolvePath(path))
	if err != nil {
		return nil, hostshell.OSErrText(err)
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.IsDir() {
		return nil, hostshell.OSErrText(syscall.EISDIR)
	}
	var src io.Reader = f
	if limit >= 0 && limit < 1<<62 {
		src = io.LimitReader(f, limit+1)
	}
	b, err := io.ReadAll(src)
	switch {
	case err != nil:
		return nil, hostshell.OSErrText(err)
	case limit >= 0 && int64(len(b)) > limit:
		return nil, fmt.Sprintf("file is larger than the read limit of %d bytes", uint64(limit))
	}
	return b, ""
}

func (m *Machine) registerFileReads() {
	h := m.host
	h["fs.Files.readBytesUpTo"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		b, bad := m.readLimited(pathText(a[1]), deref(a[2]).i)
		if bad != "" {
			return m.errMsg(bad)
		}
		return m.ok(mkBytes(b))
	}
	h["fs.Files.readBytes"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		b, bad := m.readLimited(pathText(a[1]), defaultReadLimit)
		if bad != "" {
			return m.errMsg(bad)
		}
		return m.ok(mkBytes(b))
	}
	h["fs.Files.read"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		b, bad := m.readLimited(pathText(a[1]), defaultReadLimit)
		switch {
		case bad != "":
			return m.errMsg(bad)
		case !utf8.Valid(b):
			return m.errMsg("invalid UTF-8")
		}
		return m.ok(mkStr(b))
	}
	h["fs.Files.createWithMode"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		fh, err := os.OpenFile(m.resolvePath(pathText(a[1])), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, unixMode(deref(a[2]).i))
		if err != nil {
			return m.errMsg(hostshell.OSErrText(err))
		}
		file := m.r.Types.Node(m.resultPayload(fn)).Ent
		return m.ok(mkRecord(file, []*obj{mkOpaque("Ptr", fh)}))
	}
	h["fs.Files.remove"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		path := m.resolvePath(pathText(a[1]))
		if info, err := os.Lstat(path); err == nil && info.IsDir() {
			return m.errMsg(hostshell.OSErrText(syscall.EISDIR))
		}
		if err := os.Remove(path); err != nil {
			return m.errMsg(hostshell.OSErrText(err))
		}
		return m.ok(unitObj)
	}
	h["fs.Files.removeDir"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if err := syscall.Rmdir(m.resolvePath(pathText(a[1]))); err != nil {
			return m.errMsg(hostshell.OSErrText(err))
		}
		return m.ok(unitObj)
	}
}

func unixMode(bits int64) os.FileMode {
	mode := os.FileMode(bits & 0o777)
	for bit, flag := range map[int64]os.FileMode{0o4000: os.ModeSetuid, 0o2000: os.ModeSetgid, 0o1000: os.ModeSticky} {
		if bits&bit != 0 {
			mode |= flag
		}
	}
	return mode
}

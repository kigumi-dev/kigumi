package vm

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"kigumi/internal/compiler/sem"
)

// hostFn is a host primitive or accelerator: it sees the callee so it can
// build values of the declared result type.
type hostFn func(m *Machine, fn sem.EntityID, a []*obj) *obj

// registerHost supplies the capability objects, the process arguments,
// stdin, the environment and the file system, replacing the extern(C)
// bodies of std/fs and std/os with Go accelerators.
func (m *Machine) registerHost() {
	m.host = map[string]hostFn{}
	m.registerProc()
	h := m.host
	h["os.Host.args"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Args", nil) }
	h["os.Host.files"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Files", nil) }
	h["os.Host.net"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Net", nil) }
	h["os.Host.stdin"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Stdin", nil) }
	h["os.Host.entropy"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Entropy", nil) }
	h["os.Args.len"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkInt(int64(len(m.opts.Args)), 64) }
	h["os.Args.get"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		i := deref(a[1]).i
		if i < 0 || i >= int64(len(m.opts.Args)) || !utf8.ValidString(m.opts.Args[i]) {
			return m.none()
		}
		return m.some(mkStr([]byte(m.opts.Args[i])))
	}
	h["os.Host.env"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if v, ok := os.LookupEnv(string(deref(a[1]).str())); ok && utf8.ValidString(v) {
			return m.some(mkStr([]byte(v)))
		}
		return m.none()
	}
	h["os.Args.getBytes"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		i := deref(a[1]).i
		if i < 0 || i >= int64(len(m.opts.Args)) {
			return m.none()
		}
		return m.some(mkBytes([]byte(m.opts.Args[i])))
	}
	h["os.Host.envBytes"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if v, ok := os.LookupEnv(string(deref(a[1]).str())); ok {
			return m.some(mkBytes([]byte(v)))
		}
		return m.none()
	}
	h["os.Host.chdir"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		target := m.resolvePath(string(deref(a[1]).str()))
		if err := os.Chdir(target); err != nil {
			return m.errMsg(err.Error())
		}
		if wd, err := os.Getwd(); err == nil {
			m.setMachineCwd(wd)
		} else {
			m.setMachineCwd(target)
		}
		return m.ok(unitObj)
	}
	h["os.Host.cwd"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkStr([]byte(utf8Lossy(m.machineCwd())))
	}
	h["os.Host.setEnv"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if err := os.Setenv(string(deref(a[1]).str()), string(deref(a[2]).str())); err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(unitObj)
	}
	h["os.Host.unsetEnv"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if err := os.Unsetenv(string(deref(a[1]).str())); err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(unitObj)
	}
	h["os.Stdin.readAll"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		out, err := m.stdinAll()
		if err != nil {
			return m.errMsg(err.Error())
		}
		if !utf8.Valid(out) {
			return m.errMsg("invalid UTF-8")
		}
		return m.ok(mkStr(out))
	}
	h["os.Stdin.readLine"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		line, err := m.stdinLine()
		if err != nil && err != io.EOF {
			return m.errMsg(err.Error())
		}
		if line == "" {
			return m.ok(m.none())
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if !utf8.ValidString(trimmed) {
			return m.errMsg("invalid UTF-8")
		}
		return m.ok(m.some(mkStr([]byte(trimmed))))
	}
	h["os.Host.stdout"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Stdout", nil) }
	h["os.Stdout.write"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		n, err := m.writeOut(deref(a[1]).str())
		if err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(mkInt(int64(n), m.numKind(sem.TyUsize)))
	}
	h["os.Host.stderr"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Stderr", nil) }
	h["os.Stderr.write"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		n, err := m.writeErr(deref(a[1]).str())
		if err != nil {
			return m.errMsg(err.Error())
		}
		return m.ok(mkInt(int64(n), m.numKind(sem.TyUsize)))
	}
	h["os.Stdin.read"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		max := deref(a[2]).i
		if max < 0 || uint64(max) >= maxVMAlloc {
			return m.errMsg("out of memory")
		}
		buf := make([]byte, max)
		got, err := m.stdinRead(buf)
		if err != nil && err != io.EOF {
			return m.errMsg(err.Error())
		}
		arr := deref(a[1])
		for _, b := range buf[:got] {
			arr.fields = append(arr.fields, mkInt(int64(b), 8))
		}
		return m.ok(mkInt(int64(got), m.numKind(sem.TyUsize)))
	}
	h["os.Stdin.readSome"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		b, err := m.readSome(deref(a[1]).i)
		if err != nil {
			if err == io.EOF {
				return m.errMsg("stdin is closed")
			}
			return m.errMsg(err.Error())
		}
		return m.ok(mkBytes(b))
	}
	m.registerFiles()
}

// Callers hold sh.in: the reader is machine-wide and one lane's read must not
// interleave with another's.
func (m *Machine) stdinReader() *bufio.Reader {
	if m.sh.stdin == nil {
		r := m.opts.Stdin
		if r == nil {
			r = os.Stdin
			if stdinWasClosed() {
				// The runtime substituted /dev/null for the closed descriptor: an unreadable stdin, not an empty one.
				r = errReader{syscall.EBADF}
			}
		}
		m.sh.stdinSrc = r
		m.sh.stdin = bufio.NewReader(r)
	}
	return m.sh.stdin
}

func (m *Machine) resolvePath(p string) string {
	cwd := m.machineCwd()
	if p == "" || cwd == "" || strings.HasPrefix(p, "/") {
		return p
	}
	return cwd + "/" + p
}

// absoluteCwd is the program's starting directory: Options.Cwd when set,
// the process directory otherwise, always absolute so Host.cwd answers the
// same way a native build's getcwd does.
func absoluteCwd(dir string) string {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return ""
		}
		return wd
	}
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	wd, err := os.Getwd()
	if err != nil {
		return filepath.Clean(dir)
	}
	return filepath.Clean(filepath.Join(wd, dir))
}

// machineCwd is shared, so a chdir in one lane is visible to every lane.
func (m *Machine) machineCwd() string {
	m.sh.cwdMu.RLock()
	defer m.sh.cwdMu.RUnlock()
	return m.sh.cwd
}

func (m *Machine) setMachineCwd(dir string) {
	m.sh.cwdMu.Lock()
	defer m.sh.cwdMu.Unlock()
	m.sh.cwd = dir
}

package vm

import (
	"errors"
	"net"
	"strings"
	"syscall"

	"kigumi/internal/backend/vm/host/shell"
)

// dialError renders a connect failure like native: connectSocket's poll(2) timeout wording, getaddrinfo's gai_strerror text for a failed lookup, else the errno's strerror text.
func dialError(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("connect timed out")
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return errors.New("cannot resolve host: " + gaiText(dns))
	}
	return ioError(err)
}

func gaiText(e *net.DNSError) string {
	switch {
	case e.IsNotFound:
		return "Name or service not known"
	case e.IsTemporary:
		return "Temporary failure in name resolution"
	}
	msg := e.Err
	if msg == "" {
		return "System error"
	}
	return strings.ToUpper(msg[:1]) + msg[1:]
}

// ioError renders a read/write failure like ffi.lastError(): EAGAIN's strerror text on timeout, else the errno's strerror text.
func ioError(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("Resource temporarily unavailable")
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errors.New(shell.OSErrText(errno))
	}
	return err
}

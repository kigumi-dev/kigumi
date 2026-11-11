// Package net is the VM accelerators' IPv4 TCP implementation of
// std/net's socket layer. It works directly on raw file descriptors, the
// same currency std/net/net_hosted.kg's extern(C) body uses, so a
// Conn/Listener's `fd` field means the same thing under every backend.
//
// The public API carries the descriptor as an int on every host. Unix keeps
// the descriptor as-is; Windows has to widen it to a SOCKET handle and back,
// so the conversions live in netsock_unix.go and netsock_windows.go next to
// the platform calls that need them.
package net

import (
	"errors"
	"fmt"
	stdnet "net"
	"strconv"
	"syscall"
)

// errFDOverflow reports a socket handle that cannot round-trip through the
// int the Kigumi side stores.
var errFDOverflow = errors.New("socket handle does not fit in an int")

// resultFD narrows a freshly opened handle for the public API and closes it
// when the handle cannot be represented.
func resultFD(fd osFD) (int, error) {
	n, err := apiFD(fd)
	if err != nil {
		_ = closeFD(fd)
		return -1, err
	}
	return n, nil
}

// Dial resolves host:port over IPv4 and returns a connected socket's fd.
func Dial(host string, port int64) (int, error) {
	addr, err := stdnet.ResolveTCPAddr("tcp4", stdnet.JoinHostPort(host, strconv.FormatInt(port, 10)))
	if err != nil {
		return -1, err
	}
	ip4 := addr.IP.To4()
	if ip4 == nil {
		return -1, fmt.Errorf("%s does not resolve to an IPv4 address", host)
	}
	fd, err := openSocket()
	if err != nil {
		return -1, err
	}
	sa := &syscall.SockaddrInet4{Port: addr.Port}
	copy(sa.Addr[:], ip4)
	if err := connectFD(fd, sa); err != nil {
		_ = closeFD(fd)
		return -1, err
	}
	return resultFD(fd)
}

// Listen binds host:port (SO_REUSEADDR set) over IPv4 and starts listening;
// an empty host binds every interface.
func Listen(host string, port int64) (int, error) {
	if host == "" {
		host = "0.0.0.0"
	}
	ip := stdnet.ParseIP(host)
	if ip == nil {
		addr, err := stdnet.ResolveIPAddr("ip4", host)
		if err != nil {
			return -1, err
		}
		ip = addr.IP
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return -1, fmt.Errorf("%s does not resolve to an IPv4 address", host)
	}
	fd, err := openSocket()
	if err != nil {
		return -1, err
	}
	if err := reuseAddrFD(fd); err != nil {
		_ = closeFD(fd)
		return -1, err
	}
	sa := &syscall.SockaddrInet4{Port: int(port)}
	copy(sa.Addr[:], ip4)
	if err := bindFD(fd, sa); err != nil {
		_ = closeFD(fd)
		return -1, err
	}
	if err := listenFD(fd, 16); err != nil {
		_ = closeFD(fd)
		return -1, err
	}
	return resultFD(fd)
}

// Accept blocks for the next connection on a listening fd.
func Accept(fd int) (int, error) {
	h, err := osFDOf(fd)
	if err != nil {
		return -1, err
	}
	nfd, err := acceptFD(h)
	if err != nil {
		return -1, err
	}
	return resultFD(nfd)
}

// Port is the local port a bound fd holds, or 0 when it cannot be read.
func Port(fd int) int {
	h, err := osFDOf(fd)
	if err != nil {
		return 0
	}
	sa, err := getsocknameFD(h)
	if err != nil {
		return 0
	}
	if sa4, ok := sa.(*syscall.SockaddrInet4); ok {
		return sa4.Port
	}
	return 0
}

// Read fills buf and reports how many bytes came in; 0, nil is EOF.
func Read(fd int, buf []byte) (int, error) {
	h, err := osFDOf(fd)
	if err != nil {
		return 0, err
	}
	return readFD(h, buf)
}

// Write may send fewer bytes than given, like the POSIX call it wraps.
func Write(fd int, data []byte) (int, error) {
	h, err := osFDOf(fd)
	if err != nil {
		return 0, err
	}
	return writeFD(h, data)
}

// Close closes fd; the caller reports the error or ignores it, matching
// close(2)'s two Kigumi-side callers (an explicit close and a drop).
func Close(fd int) error {
	h, err := osFDOf(fd)
	if err != nil {
		return err
	}
	return closeFD(h)
}

// TryAccept accepts without blocking; ok is false when no connection is waiting.
// Both the listener and the accepted fd are left non-blocking, as net_hosted.kg leaves them.
func TryAccept(fd int) (nfd int, ok bool, err error) {
	h, err := osFDOf(fd)
	if err != nil {
		return -1, false, err
	}
	if err := setNonblockFD(h, true); err != nil {
		return -1, false, err
	}
	for {
		accepted, err := acceptFD(h)
		switch {
		case err == nil:
			if err := setNonblockFD(accepted, true); err != nil {
				_ = closeFD(accepted)
				return -1, false, err
			}
			nfd, err := resultFD(accepted)
			if err != nil {
				return -1, false, err
			}
			return nfd, true, nil
		case wouldBlock(err):
			return -1, false, nil
		case interrupted(err):
			continue
		}
		return -1, false, err
	}
}

// TryRead reads without blocking; ok is false when no input is ready yet.
func TryRead(fd int, buf []byte) (n int, ok bool, err error) {
	h, err := osFDOf(fd)
	if err != nil {
		return 0, false, err
	}
	if err := setNonblockFD(h, true); err != nil {
		return 0, false, err
	}
	for {
		n, err = readFD(h, buf)
		switch {
		case err == nil:
			return n, true, nil
		case wouldBlock(err):
			return 0, false, nil
		case interrupted(err):
			continue
		}
		return 0, false, err
	}
}

// TryWrite sends without blocking; ok is false when the send buffer is full.
func TryWrite(fd int, data []byte) (n int, ok bool, err error) {
	h, err := osFDOf(fd)
	if err != nil {
		return 0, false, err
	}
	if err := setNonblockFD(h, true); err != nil {
		return 0, false, err
	}
	for {
		n, err = writeFD(h, data)
		switch {
		case err == nil:
			return n, true, nil
		case wouldBlock(err):
			return 0, false, nil
		case interrupted(err):
			continue
		}
		return 0, false, err
	}
}

//go:build windows

package net

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// osFD is a Winsock SOCKET. It does not fit in the int the Kigumi side stores
// on every host, so the two conversions check the round trip.
type osFD = syscall.Handle

func osFDOf(fd int) (osFD, error) {
	if fd < 0 {
		return 0, errFDOverflow
	}
	h := osFD(fd)
	if int(h) != fd {
		return 0, errFDOverflow
	}
	return h, nil
}

func apiFD(fd osFD) (int, error) {
	n := int(fd)
	if osFD(n) != fd {
		return -1, errFDOverflow
	}
	return n, nil
}

const (
	// FIONBIO is the ioctlsocket request that enables non-blocking mode.
	// syscall.SetNonblock is a no-op on Windows, so Try* cannot rely on it.
	fionbio = 0x8004667e
	// Winsock error codes are not the synthetic Errno values syscall exposes
	// for compatibility; these are the numbers the API actually returns.
	wsaEINTR       syscall.Errno = 10004
	wsaEWOULDBLOCK syscall.Errno = 10035
	// netInvalidSocket is SOCKET_ERROR, the failure return of the Winsock
	// entry points that do not report through WSAGetLastError alone.
	netInvalidSocket = ^uintptr(0)
)

var (
	ws2             = windows.NewLazySystemDLL("ws2_32.dll")
	procAccept      = ws2.NewProc("accept")
	procCloseSocket = ws2.NewProc("closesocket")
	procIoctlSocket = ws2.NewProc("ioctlsocket")
)

// socketCallError turns the last-error value of a Proc.Call into an error.
// Call reports Errno(0) for success, which is not a useful error value.
func socketCallError(err error) error {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno != 0 {
			return errno
		}
	} else if err != nil {
		return err
	}
	return syscall.EINVAL
}

func openSocket() (osFD, error) {
	return syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
}

func connectFD(fd osFD, sa syscall.Sockaddr) error { return syscall.Connect(fd, sa) }

func bindFD(fd osFD, sa syscall.Sockaddr) error { return syscall.Bind(fd, sa) }

func listenFD(fd osFD, backlog int) error { return syscall.Listen(fd, backlog) }

func acceptFD(fd osFD) (osFD, error) {
	// syscall.Accept is unimplemented on Windows (it returns EWINDOWS), so
	// call WS2_32's accept directly.
	nfd, _, err := procAccept.Call(uintptr(fd), 0, 0)
	if nfd == netInvalidSocket {
		return 0, socketCallError(err)
	}
	return osFD(nfd), nil
}

func getsocknameFD(fd osFD) (syscall.Sockaddr, error) { return syscall.Getsockname(fd) }

func readFD(fd osFD, buf []byte) (int, error) {
	if len(buf) == 0 {
		return 0, nil
	}
	if uint64(len(buf)) > uint64(^uint32(0)) {
		return 0, syscall.EINVAL
	}
	wsabuf := windows.WSABuf{Len: uint32(len(buf)), Buf: &buf[0]}
	var n, flags uint32
	if err := windows.WSARecv(windows.Handle(fd), &wsabuf, 1, &n, &flags, nil, nil); err != nil {
		return 0, err
	}
	return int(n), nil
}

func writeFD(fd osFD, data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if uint64(len(data)) > uint64(^uint32(0)) {
		return 0, syscall.EINVAL
	}
	wsabuf := windows.WSABuf{Len: uint32(len(data)), Buf: &data[0]}
	var n uint32
	if err := windows.WSASend(windows.Handle(fd), &wsabuf, 1, &n, 0, nil, nil); err != nil {
		return 0, err
	}
	return int(n), nil
}

func closeFD(fd osFD) error {
	// CloseHandle does not run Winsock's closesocket cleanup; use the socket
	// API so a closed connection enters the normal TIME_WAIT path.
	r, _, err := procCloseSocket.Call(uintptr(fd))
	if r != 0 {
		return socketCallError(err)
	}
	return nil
}

func setNonblockFD(fd osFD, nonblocking bool) error {
	var mode uint32
	if nonblocking {
		mode = 1
	}
	r, _, err := procIoctlSocket.Call(uintptr(fd), fionbio, uintptr(unsafe.Pointer(&mode)))
	if r != 0 {
		return socketCallError(err)
	}
	return nil
}

func reuseAddrFD(fd osFD) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

func errnoOf(err error) (syscall.Errno, bool) {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno, true
	}
	return 0, false
}

func wouldBlock(err error) bool {
	errno, ok := errnoOf(err)
	return ok && (errno == wsaEWOULDBLOCK || errno == syscall.EAGAIN || errno == syscall.EWOULDBLOCK)
}

func interrupted(err error) bool {
	errno, ok := errnoOf(err)
	return ok && (errno == wsaEINTR || errno == syscall.EINTR)
}

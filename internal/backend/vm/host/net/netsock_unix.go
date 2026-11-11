//go:build linux || darwin || freebsd || netbsd || openbsd

package net

import "syscall"

// osFD is the platform's descriptor type. Unix descriptors already are ints,
// so it stays an alias and the conversions below are identities.
type osFD = int

func osFDOf(fd int) (osFD, error) { return fd, nil }

func apiFD(fd osFD) (int, error) { return fd, nil }

func openSocket() (osFD, error) {
	return syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
}

func connectFD(fd osFD, sa syscall.Sockaddr) error { return syscall.Connect(fd, sa) }

func bindFD(fd osFD, sa syscall.Sockaddr) error { return syscall.Bind(fd, sa) }

func listenFD(fd osFD, backlog int) error { return syscall.Listen(fd, backlog) }

func acceptFD(fd osFD) (osFD, error) {
	nfd, _, err := syscall.Accept(fd)
	return nfd, err
}

func getsocknameFD(fd osFD) (syscall.Sockaddr, error) { return syscall.Getsockname(fd) }

func readFD(fd osFD, buf []byte) (int, error) { return syscall.Read(fd, buf) }

func writeFD(fd osFD, data []byte) (int, error) { return syscall.Write(fd, data) }

func closeFD(fd osFD) error { return syscall.Close(fd) }

func setNonblockFD(fd osFD, nonblocking bool) error {
	return syscall.SetNonblock(fd, nonblocking)
}

func reuseAddrFD(fd osFD) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}

func wouldBlock(err error) bool {
	return err == syscall.EAGAIN || err == syscall.EWOULDBLOCK
}

func interrupted(err error) bool { return err == syscall.EINTR }

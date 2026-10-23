package vm

import (
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"kigumi/internal/sem"
)

// httpRequestTimeout is re-armed before every read/write, matching net_hosted.kg's SO_RCVTIMEO/SO_SNDTIMEO (per-syscall, not overall).
const httpRequestTimeout = 10 * time.Second

// registerNetHTTP installs Net.http/Http.post over net.Conn instead of running net_hosted.kg's extern(C) body, which the VM cannot call.
func (m *Machine) registerNetHTTP() {
	h := m.host
	h["net.Net.http"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj { return mkOpaque("Http", nil) }
	h["net.Http.post"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		status, err := httpPost(string(deref(a[1]).str()), deref(a[2]).str())
		if err != nil {
			return m.errMsg(err.Error())
		}
		resp := m.r.Types.Node(m.resultPayload(fn)).Ent
		return m.ok(mkRecord(resp, []*obj{mkInt(int64(status), m.numKind(sem.TyI64))}))
	}
}

func httpPost(rawURL string, body []byte) (int, error) {
	host, header, port, path, err := parseHTTPURL(rawURL)
	if err != nil {
		return 0, err
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), httpRequestTimeout)
	if err != nil {
		return 0, dialTimeoutError(err)
	}
	defer conn.Close()
	req := "POST " + path + " HTTP/1.1\r\nHost: " + header + "\r\nContent-Type: text/plain\r\nContent-Length: " +
		strconv.Itoa(len(body)) + "\r\nConnection: close\r\n\r\n"
	if err := writeHTTP(conn, []byte(req)); err != nil {
		return 0, err
	}
	if err := writeHTTP(conn, body); err != nil {
		return 0, err
	}
	line, err := readHTTPStatusLine(conn)
	if err != nil {
		return 0, err
	}
	status, ok := parseHTTPStatus(line)
	if !ok {
		return 0, errors.New("malformed response")
	}
	return status, nil
}

// writeHTTP re-arms the deadline per write like SO_SNDTIMEO: a slow peer that keeps accepting bytes never times out.
func writeHTTP(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(httpRequestTimeout)); err != nil {
			return err
		}
		n, err := conn.Write(data)
		data = data[n:]
		if err != nil {
			var ne net.Error
			if n > 0 && errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return ioError(err)
		}
	}
	return nil
}

// dialTimeoutError matches connectSocket's poll(2) timeout wording; native never reaches SO_RCVTIMEO/SO_SNDTIMEO until after connect() succeeds.
func dialTimeoutError(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("connect timed out")
	}
	return err
}

// ioError renders a read/write failure like ffi.lastError(): EAGAIN's strerror text on timeout, else the errno's message capitalized to match strerror (Go's is lowercase).
func ioError(err error) error {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return errors.New("Resource temporarily unavailable")
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg := errno.Error()
		return errors.New(strings.ToUpper(msg[:1]) + msg[1:])
	}
	return err
}

// readHTTPStatusLine reads up to LF or 64 bytes, whichever comes first, matching net_hosted.kg's readLine byte-at-a-time cap.
func readHTTPStatusLine(conn net.Conn) ([]byte, error) {
	var acc []byte
	buf := make([]byte, 1)
	for len(acc) < 64 {
		if err := conn.SetReadDeadline(time.Now().Add(httpRequestTimeout)); err != nil {
			return nil, err
		}
		n, err := conn.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			acc = append(acc, buf[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, ioError(err)
		}
		if n == 0 {
			break
		}
	}
	return acc, nil
}

// parseHTTPStatus matches statusOf: "HTTP/1.1 200 OK" is the second space-separated word, exactly three ASCII digits.
func parseHTTPStatus(line []byte) (int, bool) {
	if !strings.HasPrefix(string(line), "HTTP/") {
		return 0, false
	}
	parts := strings.Split(string(line), " ")
	if len(parts) < 2 || len(parts[1]) != 3 {
		return 0, false
	}
	n := 0
	for i := 0; i < 3; i++ {
		d := parts[1][i]
		if d < '0' || d > '9' {
			return 0, false
		}
		n = n*10 + int(d-'0')
	}
	return n, true
}

package interp

import (
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// httpRequestTimeout is re-armed before every read/write, matching
// net_hosted.kg's SO_RCVTIMEO/SO_SNDTIMEO (per-syscall, not overall).
const httpRequestTimeout = 10 * time.Second

// httpPost sends a raw request over net.Conn instead of net/http, so it sends the same bytes native/VM do and rejects https too.
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

// dialTimeoutError matches connectSocket's poll(2) timeout wording; native
// never reaches SO_RCVTIMEO/SO_SNDTIMEO until after connect() succeeds.
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

// parseHTTPURL mirrors Http.post's own validation (the path, the host and
// the authority it splits into): same checks, same error text.
func parseHTTPURL(rawURL string) (host, header, port, path string, err error) {
	if strings.HasPrefix(rawURL, "https://") {
		return "", "", "", "", errors.New("https is not supported in native builds")
	}
	if !strings.HasPrefix(rawURL, "http://") {
		return "", "", "", "", errors.New("unsupported URL scheme")
	}
	rest := rawURL[len("http://"):]
	authority, path := rest, "/"
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		authority, path = rest[:i], rest[i:]
	}
	if hasUnsafeHTTPByte(path) {
		return "", "", "", "", errors.New("path contains a control character or a space")
	}
	if hasUnsafeHTTPByte(authority) {
		return "", "", "", "", errors.New("host contains a control character or a space")
	}
	host, header, port, err = parseHTTPAuthority(authority)
	return host, header, port, path, err
}

func parseHTTPAuthority(authority string) (host, header, port string, err error) {
	if strings.Contains(authority, "@") {
		return "", "", "", errors.New("userinfo in URL is not supported")
	}
	if strings.HasPrefix(authority, "[") {
		end := strings.IndexByte(authority, ']')
		if end < 0 {
			return "", "", "", errors.New("malformed IPv6 host")
		}
		host = authority[1:end]
		switch after := authority[end+1:]; {
		case after == "":
			port = "80"
		case strings.HasPrefix(after, ":"):
			port = after[1:]
		default:
			return "", "", "", errors.New("malformed IPv6 host")
		}
		return host, "[" + host + "]", port, nil
	}
	host, port = authority, "80"
	if i := strings.IndexByte(authority, ':'); i >= 0 {
		host, port = authority[:i], authority[i+1:]
	}
	return host, host, port, nil
}

// readHTTPStatusLine reads up to LF or 64 bytes, whichever comes first,
// matching net_hosted.kg's readLine byte-at-a-time cap.
func readHTTPStatusLine(conn net.Conn) (string, error) {
	var acc []byte
	buf := make([]byte, 1)
	for len(acc) < 64 {
		if err := conn.SetReadDeadline(time.Now().Add(httpRequestTimeout)); err != nil {
			return "", err
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
			return "", ioError(err)
		}
		if n == 0 {
			break
		}
	}
	if !utf8.Valid(acc) {
		return "", errors.New("invalid UTF-8")
	}
	return string(acc), nil
}

// parseHTTPStatus matches statusOf: "HTTP/1.1 200 OK" is the second
// space-separated word, exactly three ASCII digits.
func parseHTTPStatus(line string) (int, bool) {
	if !strings.HasPrefix(line, "HTTP/") {
		return 0, false
	}
	parts := strings.Split(line, " ")
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

func hasUnsafeHTTPByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] <= 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

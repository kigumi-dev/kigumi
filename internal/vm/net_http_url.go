package vm

import (
	"errors"
	"strings"
)

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

func hasUnsafeHTTPByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] <= 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}

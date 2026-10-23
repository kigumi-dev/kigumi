package vm

import (
	"net"
	"syscall"
	"testing"
)

func TestDialErrorUsesNativeWording(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"refused", &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}, "Connection refused"},
		{"not found", &net.DNSError{Err: "no such host", Name: "x.invalid", IsNotFound: true}, "cannot resolve host: Name or service not known"},
		{"temporary", &net.DNSError{Err: "server misbehaving", IsTemporary: true}, "cannot resolve host: Temporary failure in name resolution"},
		{"other lookup", &net.DNSError{Err: "bad query"}, "cannot resolve host: Bad query"},
		{"timeout", &net.OpError{Op: "dial", Err: timeoutErr{}}, "connect timed out"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dialError(tc.err); got.Error() != tc.want {
				t.Fatalf("dialError = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDialErrorOfARefusedConnection(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	addr := l.Addr().String()
	l.Close()
	_, err = net.Dial("tcp", addr)
	if err == nil {
		t.Skip("port was reused")
	}
	if got := dialError(err); got.Error() != "Connection refused" {
		t.Fatalf("dialError = %q", got)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

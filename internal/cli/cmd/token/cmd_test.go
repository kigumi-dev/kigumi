package token_test

import (
	"testing"

	"kigumi/internal/cli/clitest"
)

func TestTokenCommand(t *testing.T) {
	for _, tt := range []struct {
		word string
		want string
	}{
		{"fn", "fn\n"},
		{"static", "identifier\n"},
	} {
		code, out, errOut := clitest.Execute(t, "token", tt.word)
		if code != 0 {
			t.Fatalf("token %s: exit %d, stderr %q", tt.word, code, errOut)
		}
		if out != tt.want {
			t.Errorf("token %s = %q, want %q", tt.word, out, tt.want)
		}
	}
}

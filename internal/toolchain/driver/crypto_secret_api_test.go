package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestCryptoSecretAPIsRejectPublicBytes is the structural negative test for
// the Secret migration: a raw key, or a raw plaintext, cannot be passed where
// the AEAD/HMAC APIs require a SecretBuffer. The program is expected not to
// compile; accepting it would silently reintroduce a public key path.
func TestCryptoSecretAPIsRejectPublicBytes(t *testing.T) {
	t.Parallel()
	std, _ := filepath.Abs("../../../std")
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "seal-raw-key",
			src: `import aead from std/crypto/chacha20poly1305
let rawKey = Bytes.fill(0, 32)
let plaintext = Bytes.fill(1, 4)
let _ = aead.seal(rawKey, Bytes.zeros(12), Bytes.zeros(0), plaintext)
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
		{
			name: "open-raw-key",
			src: `import aead from std/crypto/chacha20poly1305
let rawKey = Bytes.fill(0, 32)
let _ = aead.open(rawKey, Bytes.zeros(12), Bytes.zeros(0), Bytes.zeros(20))
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
		{
			name: "seal-raw-plaintext",
			src: `import aead from std/crypto/chacha20poly1305
import secret from std/secret
let key = secret.SecretBuffer.fromBytes(Bytes.fill(0, 32))
let plaintext = Bytes.fill(1, 4)
let _ = aead.seal(&key, Bytes.zeros(12), Bytes.zeros(0), plaintext)
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
		{
			name: "hmac-raw-key",
			src: `import hmac from std/crypto/hmac
import sha256 from std/crypto/sha256
let msg = Bytes.fill(1, 4)
let _ = hmac.sum[sha256.SecretHasher](Bytes.fill(0, 32), msg)
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"ct\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(tc.src), 0o644)
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main.kg"})
			var stderr bytes.Buffer
			ok := false
			if err == nil {
				_, ok, err = driver.Compile(m, &stderr)
			}
			if ok {
				t.Fatalf("compiled a program that passes a public value as secret:\n%s", tc.src)
			}
			if err == nil && !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("diagnostic does not name the Secret boundary\n%s", stderr.String())
			}
		})
	}
}

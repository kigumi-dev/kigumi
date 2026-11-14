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
			name: "x25519-public-raw-scalar",
			src: `import x25519 from std/crypto/x25519
let _ = x25519.publicKey(Bytes.zeros(32))
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
		{
			name: "x25519-shared-raw-scalar",
			src: `import x25519 from std/crypto/x25519
let _ = x25519.sharedSecret(Bytes.zeros(32), Bytes.fill(9, 32))
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
		{
			name: "p256-sign-raw-scalar",
			src: `import p256 from std/crypto/p256
let _ = p256.sign(Bytes.zeros(32), Bytes.zeros(32))
`,
			want: "expected `&SecretBuffer`, found `Bytes`",
		},
		{
			name: "p256-sign-message-raw-scalar",
			src: `import p256 from std/crypto/p256
let _ = p256.signMessage(Bytes.zeros(32), Bytes.zeros(4))
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

// TestPKSecretPathsRunUnderSecretSan runs the migrated public-key secret
// paths on the VM with the shadow checker. The private scalars enter as
// SecretBuffers, the shared secret and signature are released only at the
// explicit test boundary, and no secret reaches a sink.
func TestPKSecretPathsRunUnderSecretSan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"pk\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(`import p256 from std/crypto/p256
import secret from std/secret
import x25519 from std/crypto/x25519

fn main() {
    let a = secret.SecretBuffer.fromBytes(Bytes.fill(7, 32))
    let b = secret.SecretBuffer.fromBytes(Bytes.fill(9, 32))
    let aPub = x25519.publicKey(&a).expect("public")
    let bPub = x25519.publicKey(&b).expect("public")
    let ab = x25519.sharedSecret(&a, bPub).expect("shared")
    let ba = x25519.sharedSecret(&b, aPub).expect("shared")
    let same = unsafe { ab.ctEq(&ba).declassify() }
    let sig = p256.signMessage(&a, "message".toBytes()).expect("sign")
    print "${same} ${sig.len()}"
}
`), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{Cwd: t.TempDir(), SecretSan: true}, &out, &errOut, []string{"main.kg"})
	if err != nil {
		t.Fatalf("--secret-san: %v\n%s", err, errOut.String())
	}
	if code != 0 || out.String() != "true 64\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, out.String(), errOut.String())
	}
}

// Package zigrelease signs fake releases the way minisign does, standing in
// for ziglang.org in tests; nothing in the kigumi binary imports it.
package zigrelease

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"kigumi/internal/toolchain/zig/fetch"
)

// NewSigningKey makes a throwaway Ed25519 key pair and its minisign
// public-key line, for use as fetch.Options.PublicKey.
func NewSigningKey() (priv ed25519.PrivateKey, keyID [8]byte, publicKeyLine string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	if _, err := rand.Read(keyID[:]); err != nil {
		panic(err)
	}
	raw := append(append([]byte("Ed"), keyID[:]...), pub...)
	return priv, keyID, base64.StdEncoding.EncodeToString(raw)
}

// SignFile writes a .minisig in minisign's "ED" (BLAKE2b-512-prehashed) form,
// with the same global-signature trusted-comment scheme.
func SignFile(priv ed25519.PrivateKey, keyID [8]byte, data []byte, trustedComment string) []byte {
	digest := fetch.Blake2b512Sum(data)
	sig := ed25519.Sign(priv, digest[:])
	blob := append(append([]byte("ED"), keyID[:]...), sig...)
	globalMsg := append(append([]byte{}, sig...), []byte(trustedComment)...)
	globalSig := ed25519.Sign(priv, globalMsg)
	return fmt.Appendf(nil, "untrusted comment: signature from minisign secret key\n%s\ntrusted comment: %s\n%s\n",
		base64.StdEncoding.EncodeToString(blob), trustedComment, base64.StdEncoding.EncodeToString(globalSig))
}

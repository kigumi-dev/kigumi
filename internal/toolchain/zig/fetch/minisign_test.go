package fetch

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"testing"
)

func newTestKey(t *testing.T) (priv ed25519.PrivateKey, keyID [8]byte, pubLine string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(keyID[:]); err != nil {
		t.Fatal(err)
	}
	raw := append(append([]byte("Ed"), keyID[:]...), pub...)
	return priv, keyID, base64.StdEncoding.EncodeToString(raw)
}

func TestVerifyMinisignRejectsMissingGlobalSignature(t *testing.T) {
	data := []byte("probe data")
	priv, keyID, pubLine := newTestKey(t)
	digest := Blake2b512Sum(data)
	primary := ed25519.Sign(priv, digest[:])
	blob := append(append([]byte("ED"), keyID[:]...), primary...)
	noGlobalSig := fmt.Appendf(nil, "untrusted comment: probe\n%s\n",
		base64.StdEncoding.EncodeToString(blob))

	err := verifyMinisign(data, noGlobalSig, pubLine)
	if err == nil {
		t.Fatal("a .minisig with no trusted-comment/global-signature block must not verify")
	}
}

func TestVerifyMinisignRejectsForgedGlobalSignature(t *testing.T) {
	data := []byte("probe data")
	priv, keyID, pubLine := newTestKey(t)
	digest := Blake2b512Sum(data)
	primary := ed25519.Sign(priv, digest[:])
	blob := append(append([]byte("ED"), keyID[:]...), primary...)

	otherPub, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	_ = otherPub
	if err != nil {
		t.Fatal(err)
	}
	trustedComment := "timestamp:1 file:probe"
	globalMsg := append(append([]byte{}, primary...), []byte(trustedComment)...)
	forgedGlobalSig := ed25519.Sign(otherPriv, globalMsg)

	blobWithForgedGlobal := fmt.Appendf(nil, "untrusted comment: probe\n%s\ntrusted comment: %s\n%s\n",
		base64.StdEncoding.EncodeToString(blob), trustedComment,
		base64.StdEncoding.EncodeToString(forgedGlobalSig))

	err = verifyMinisign(data, blobWithForgedGlobal, pubLine)
	if err == nil {
		t.Fatal("a .minisig whose global signature is forged with a different key must not verify")
	}
}

func TestVerifyMinisignAcceptsWellFormedSignature(t *testing.T) {
	data := []byte("probe data")
	priv, keyID, pubLine := newTestKey(t)
	digest := Blake2b512Sum(data)
	primary := ed25519.Sign(priv, digest[:])
	blob := append(append([]byte("ED"), keyID[:]...), primary...)
	trustedComment := "timestamp:1 file:probe"
	globalMsg := append(append([]byte{}, primary...), []byte(trustedComment)...)
	globalSig := ed25519.Sign(priv, globalMsg)

	wellFormed := fmt.Appendf(nil, "untrusted comment: probe\n%s\ntrusted comment: %s\n%s\n",
		base64.StdEncoding.EncodeToString(blob), trustedComment,
		base64.StdEncoding.EncodeToString(globalSig))

	if err := verifyMinisign(data, wellFormed, pubLine); err != nil {
		t.Fatalf("well-formed signature with a global signature must verify: %v", err)
	}
}

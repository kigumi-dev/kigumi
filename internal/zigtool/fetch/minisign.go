package fetch

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"strings"
)

// zigPublicKey is the Zig Software Foundation's published minisign key
// (https://ziglang.org/download/), pinned here for offline verification.
const zigPublicKey = "RWSGOq2NVecA2UPNdBUZykf1CCb147pkmdtYxgb3Ti+JO/wCYvhbAb/U"

// minisignPublicKey is a parsed "minisign public key" line: 2-byte
// algorithm ("Ed"), 8-byte key id, 32-byte Ed25519 public key.
type minisignPublicKey struct {
	keyID [8]byte
	pub   ed25519.PublicKey
}

func parseMinisignPublicKey(encoded string) (minisignPublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return minisignPublicKey{}, fmt.Errorf("minisign public key: %w", err)
	}
	if len(raw) != 42 || string(raw[:2]) != "Ed" {
		return minisignPublicKey{}, fmt.Errorf("minisign public key: want 42 bytes starting \"Ed\", got %d bytes", len(raw))
	}
	var k minisignPublicKey
	copy(k.keyID[:], raw[2:10])
	k.pub = append(ed25519.PublicKey{}, raw[10:42]...)
	return k, nil
}

// minisignSignature is one parsed .minisig: the detached signature over the
// file (digest, for "ED"), plus the trusted comment and global signature over it, which modern minisign always writes.
type minisignSignature struct {
	hashed      bool
	keyID       [8]byte
	sig         [64]byte
	trustedLine string
	globalSig   [64]byte
}

func parseMinisignSignature(data []byte) (minisignSignature, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var sig minisignSignature
	var blobLine, trustedLine string
	var globalLine string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "untrusted comment:"):
			if i+1 >= len(lines) {
				return sig, fmt.Errorf("minisign signature: missing signature line")
			}
			blobLine = lines[i+1]
			i++
		case strings.HasPrefix(line, "trusted comment:"):
			trustedLine = strings.TrimPrefix(line, "trusted comment:")
			trustedLine = strings.TrimPrefix(trustedLine, " ")
			if i+1 < len(lines) {
				globalLine = lines[i+1]
			}
			i++
		}
	}
	if blobLine == "" {
		return sig, fmt.Errorf("minisign signature: no signature blob found")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(blobLine))
	if err != nil {
		return sig, fmt.Errorf("minisign signature: %w", err)
	}
	if len(raw) != 74 {
		return sig, fmt.Errorf("minisign signature: want 74 bytes (algorithm+keyid+signature), got %d", len(raw))
	}
	switch string(raw[:2]) {
	case "Ed":
		sig.hashed = false
	case "ED":
		sig.hashed = true
	default:
		return sig, fmt.Errorf("minisign signature: unknown algorithm %q", raw[:2])
	}
	copy(sig.keyID[:], raw[2:10])
	copy(sig.sig[:], raw[10:74])
	if trustedLine == "" {
		return sig, fmt.Errorf("minisign signature: missing trusted comment / global signature block")
	}
	g := strings.TrimSpace(globalLine)
	if g == "" {
		return sig, fmt.Errorf("minisign signature: missing global signature line")
	}
	gr, err := base64.StdEncoding.DecodeString(g)
	if err != nil {
		return sig, fmt.Errorf("minisign signature: global signature: %w", err)
	}
	if len(gr) != 64 {
		return sig, fmt.Errorf("minisign signature: global signature: want 64 bytes, got %d", len(gr))
	}
	sig.trustedLine = trustedLine
	copy(sig.globalSig[:], gr)
	return sig, nil
}

// verifyMinisign checks file against sigData for publicKey (zigPublicKey
// when ""): the detached "ED" signature, then the global signature over it plus the trusted comment.
func verifyMinisign(file, sigData []byte, publicKey string) error {
	if publicKey == "" {
		publicKey = zigPublicKey
	}
	key, err := parseMinisignPublicKey(publicKey)
	if err != nil {
		return err
	}
	sig, err := parseMinisignSignature(sigData)
	if err != nil {
		return err
	}
	if sig.keyID != key.keyID {
		return fmt.Errorf("minisign signature: key id %x does not match the pinned key %x", sig.keyID, key.keyID)
	}
	message := file
	if sig.hashed {
		digest := Blake2b512Sum(file)
		message = digest[:]
	}
	if !ed25519.Verify(key.pub, message, sig.sig[:]) {
		return fmt.Errorf("minisign signature: signature does not verify against the pinned key")
	}
	globalMsg := append(append([]byte{}, sig.sig[:]...), []byte(sig.trustedLine)...)
	if !ed25519.Verify(key.pub, globalMsg, sig.globalSig[:]) {
		return fmt.Errorf("minisign signature: trusted comment's global signature does not verify")
	}
	return nil
}

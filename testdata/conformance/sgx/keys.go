package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"io"
	"math/big"
	"time"
)

type signer struct{ key *ecdsa.PrivateKey }

func (s signer) Public() crypto.PublicKey { return s.key.Public() }

func (s signer) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return s.key.Sign(nil, digest, opts)
}

func fixedKey(label string, curve elliptic.Curve) *ecdsa.PrivateKey {
	digest := sha256.Sum256([]byte("kigumi SGX fixture " + label))
	d := new(big.Int).SetBytes(digest[:])
	d.Mod(d, new(big.Int).Sub(curve.Params().N, big.NewInt(1)))
	d.Add(d, big.NewInt(1))
	x, y := curve.ScalarBaseMult(d.Bytes())
	return &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: curve, X: x, Y: y}, D: d}
}

func certificate(serial int64, name string, key *ecdsa.PrivateKey, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey, ca bool, pathLen int) []byte {
	return editedCertificate(serial, name, key, issuer, issuerKey, ca, pathLen, nil)
}

func editedCertificate(serial int64, name string, key *ecdsa.PrivateKey, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey, ca bool, pathLen int, edit func(*x509.Certificate)) []byte {
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		IsCA:      ca, BasicConstraintsValid: true, MaxPathLen: pathLen, MaxPathLenZero: ca && pathLen == 0,
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	if ca {
		template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	}
	if edit != nil {
		edit(template)
	}
	if issuer == nil {
		issuer = template
		issuerKey = key
	}
	return must(x509.CreateCertificate(nil, template, issuer, &key.PublicKey, signer{issuerKey}))
}

func pemCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

type material struct {
	attest, leaf                       *ecdsa.PrivateKey
	chain, root, wrongRoot, p384Chain  []byte
	p384Root, leafDer, intermediatePem []byte
}

func makeMaterial() material {
	rootKey := fixedKey("root", elliptic.P256())
	rootDer := certificate(1, "SGX fixture root", rootKey, nil, nil, true, 1)
	root := must(x509.ParseCertificate(rootDer))
	interKey := fixedKey("intermediate", elliptic.P256())
	interDer := certificate(2, "SGX fixture intermediate", interKey, root, rootKey, true, 0)
	inter := must(x509.ParseCertificate(interDer))
	leafKey := fixedKey("leaf", elliptic.P256())
	leafDer := certificate(3, "SGX fixture leaf", leafKey, inter, interKey, false, -1)
	wrongKey := fixedKey("wrong root", elliptic.P256())
	wrong := certificate(4, "SGX fixture wrong root", wrongKey, nil, nil, true, 1)
	p384 := fixedKey("p384", elliptic.P384())
	p384Der := certificate(5, "SGX fixture P384", p384, nil, nil, true, 1)
	return material{
		attest: fixedKey("attestation", elliptic.P256()), leaf: leafKey,
		chain: append(pemCert(leafDer), pemCert(interDer)...), root: pemCert(rootDer),
		wrongRoot: pemCert(wrong), p384Chain: append(pemCert(p384Der), pemCert(p384Der)...),
		p384Root: pemCert(p384Der), leafDer: leafDer, intermediatePem: pemCert(interDer),
	}
}

func sign(key *ecdsa.PrivateKey, message []byte) []byte {
	digest := sha256.Sum256(message)
	der := must(key.Sign(nil, digest[:], crypto.SHA256))
	var sig struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &sig); err != nil {
		panic(err)
	}
	out := make([]byte, 64)
	sig.R.FillBytes(out[:32])
	sig.S.FillBytes(out[32:])
	return out
}

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"
)

var (
	oidSgxExtension = asn1.ObjectIdentifier{1, 2, 840, 113741, 1, 13, 1}
	oidEcdsaSha256  = asn1.ObjectIdentifier{1, 2, 840, 10045, 4, 3, 2}
	oidCrlNumber    = asn1.ObjectIdentifier{2, 5, 29, 20}
	oidAuthorityKey = asn1.ObjectIdentifier{2, 5, 29, 35}
	oidReasonCode   = asn1.ObjectIdentifier{2, 5, 29, 21}
	oidIssuingPoint = asn1.ObjectIdentifier{2, 5, 29, 28}
)

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func der(value any) []byte { return must(asn1.Marshal(value)) }

func derSequence(parts ...[]byte) []byte {
	var body []byte
	for _, part := range parts {
		body = append(body, part...)
	}
	return der(asn1.RawValue{Class: asn1.ClassUniversal, Tag: asn1.TagSequence, IsCompound: true, Bytes: body})
}

func sgxOid(arcs ...int) asn1.ObjectIdentifier {
	return append(append(asn1.ObjectIdentifier{}, oidSgxExtension...), arcs...)
}

type platformTcb struct {
	components [16]byte
	pceSvn     int
	fmspc      []byte
	pceID      []byte
}

type sgxExtensionEdit struct {
	dropFmspc bool
	wideSvn   bool
	repeated  bool
	platform  bool
}

func sgxExtension(tcb platformTcb, edit sgxExtensionEdit) []byte {
	var tcbParts [][]byte
	for i, svn := range tcb.components {
		value := der(int(svn))
		if edit.wideSvn && i == 0 {
			value = der(256)
		}
		tcbParts = append(tcbParts, derSequence(der(sgxOid(2, i+1)), value))
	}
	tcbParts = append(tcbParts, derSequence(der(sgxOid(2, 17)), der(tcb.pceSvn)))
	tcbParts = append(tcbParts, derSequence(der(sgxOid(2, 18)), der(tcb.components[:])))
	members := [][]byte{
		derSequence(der(sgxOid(1)), der([]byte("kigumi fixture p"))),
		derSequence(der(sgxOid(2)), derSequence(tcbParts...)),
		derSequence(der(sgxOid(3)), der(tcb.pceID)),
	}
	if !edit.dropFmspc {
		members = append(members, derSequence(der(sgxOid(4)), der(tcb.fmspc)))
	} else {
		members = append(members, derSequence(der(sgxOid(6)), der([]byte("platform instance"))))
	}
	members = append(members, derSequence(der(sgxOid(5)), der(asn1.Enumerated(0))))
	if edit.repeated {
		members = append(members, derSequence(der(sgxOid(3)), der(tcb.pceID)), derSequence(der(sgxOid(7)), derSequence()))
	}
	if edit.platform {
		members = append(members, derSequence(der(sgxOid(6)), der([]byte("platform instance"))), derSequence(der(sgxOid(7)), derSequence()))
	}
	return derSequence(members...)
}

type authority struct {
	name string
	key  *ecdsa.PrivateKey
	cert *x509.Certificate
	der  []byte
}

type certSpec struct {
	serial    int64
	name      string
	key       string
	ca        bool
	pathLen   int
	notBefore time.Time
	notAfter  time.Time
	extra     []pkix.Extension
}

func issue(spec certSpec, issuer *authority) *authority {
	key := fixedKey(spec.key, elliptic.P256())
	template := &x509.Certificate{
		SerialNumber: big.NewInt(spec.serial), Subject: pkix.Name{CommonName: spec.name, Organization: []string{"Kigumi Fixtures"}},
		NotBefore: spec.notBefore, NotAfter: spec.notAfter,
		IsCA: spec.ca, BasicConstraintsValid: true, MaxPathLen: spec.pathLen, MaxPathLenZero: spec.ca && spec.pathLen == 0,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment, ExtraExtensions: spec.extra,
	}
	if spec.ca {
		template.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
		digest := sha256.Sum256(elliptic.Marshal(elliptic.P256(), key.X, key.Y))
		template.SubjectKeyId = digest[:20]
	}
	parent, parentKey := template, key
	if issuer != nil {
		parent, parentKey = issuer.cert, issuer.key
	}
	raw := must(x509.CreateCertificate(nil, template, parent, &key.PublicKey, signer{parentKey}))
	return &authority{name: spec.name, key: key, cert: must(x509.ParseCertificate(raw)), der: raw}
}

type revocation struct {
	serial *big.Int
	reason int
}

type crlSpec struct {
	thisUpdate, nextUpdate time.Time
	revoked                []revocation
	noNumber, critical     bool
	signer                 *authority
}

// crl is built by hand because crypto/x509 cannot omit the CRL Number.
func crl(issuer *authority, spec crlSpec) []byte {
	type entry struct {
		Serial     *big.Int
		Revoked    time.Time `asn1:"utc"`
		Extensions []pkix.Extension
	}
	type tbsList struct {
		Version    int
		Algorithm  pkix.AlgorithmIdentifier
		Issuer     asn1.RawValue
		ThisUpdate time.Time        `asn1:"utc"`
		NextUpdate time.Time        `asn1:"utc"`
		Revoked    []entry          `asn1:"optional"`
		Extensions []pkix.Extension `asn1:"tag:0,optional,explicit"`
	}
	var extensions []pkix.Extension
	if !spec.noNumber {
		extensions = append(extensions, pkix.Extension{Id: oidCrlNumber, Value: der(big.NewInt(1))})
	}
	keyID := struct {
		ID []byte `asn1:"optional,tag:0"`
	}{issuer.cert.SubjectKeyId}
	extensions = append(extensions, pkix.Extension{Id: oidAuthorityKey, Value: der(keyID)})
	if spec.critical {
		extensions = append(extensions, pkix.Extension{Id: oidIssuingPoint, Critical: true, Value: derSequence()})
	}
	var entries []entry
	for _, r := range spec.revoked {
		entries = append(entries, entry{Serial: r.serial, Revoked: spec.thisUpdate, Extensions: []pkix.Extension{{Id: oidReasonCode, Value: der(asn1.Enumerated(r.reason))}}})
	}
	tbs := der(tbsList{
		Version: 1, Algorithm: pkix.AlgorithmIdentifier{Algorithm: oidEcdsaSha256},
		Issuer: asn1.RawValue{FullBytes: issuer.cert.RawSubject}, ThisUpdate: spec.thisUpdate, NextUpdate: spec.nextUpdate,
		Revoked: entries, Extensions: extensions,
	})
	key := issuer.key
	if spec.signer != nil {
		key = spec.signer.key
	}
	digest := sha256.Sum256(tbs)
	signature := must(key.Sign(nil, digest[:], crypto.SHA256))
	return derSequence(tbs, der(pkix.AlgorithmIdentifier{Algorithm: oidEcdsaSha256}), der(asn1.BitString{Bytes: signature, BitLength: len(signature) * 8}))
}

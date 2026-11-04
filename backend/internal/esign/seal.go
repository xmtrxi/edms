package esign

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/digitorus/pdf"
	"github.com/digitorus/pdfsign/sign"
)

// Sealer applies the installation's digital signature to finished documents,
// so any later change to the file is detectable.
type Sealer struct {
	key  crypto.Signer
	cert *x509.Certificate
	tsa  string
}

// NewSealer loads a PEM certificate and private key.
func NewSealer(certPEM, keyPEM []byte, tsaURL string) (*Sealer, error) {
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, errors.New("seal certificate is not PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse seal certificate: %w", err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, errors.New("seal key is not PEM")
	}
	key, err := parseKey(keyBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return &Sealer{key: key, cert: cert, tsa: tsaURL}, nil
}

func parseKey(der []byte) (crypto.Signer, error) {
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if signer, ok := key.(crypto.Signer); ok {
			return signer, nil
		}
	}
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	return nil, errors.New("unsupported seal key")
}

// SelfSigned creates a certificate and key for installations without one.
// Readers will show the seal as untrusted, but tampering is still detected.
func SelfSigned(commonName string) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return nil, nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName, Organization: []string{commonName}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
}

// SealInfo describes the seal shown in PDF readers.
type SealInfo struct {
	Name     string
	Reason   string
	Location string
	Contact  string
}

// Seal adds a certification signature that allows no further changes.
func (s *Sealer) Seal(src []byte, info SealInfo) ([]byte, error) {
	reader := bytes.NewReader(src)
	rdr, err := pdf.NewReader(reader, int64(len(src)))
	if err != nil {
		return nil, fmt.Errorf("read document to seal: %w", err)
	}
	var out bytes.Buffer
	err = sign.Sign(reader, &out, rdr, int64(len(src)), sign.SignData{
		Signature: sign.SignDataSignature{
			Info: sign.SignDataSignatureInfo{
				Name: info.Name, Reason: info.Reason, Location: info.Location,
				ContactInfo: info.Contact, Date: time.Now(),
			},
			CertType:   sign.CertificationSignature,
			DocMDPPerm: sign.DoNotAllowAnyChangesPerms,
		},
		Signer:          s.key,
		DigestAlgorithm: crypto.SHA256,
		Certificate:     s.cert,
		TSA:             sign.TSA{URL: s.tsa},
	})
	if err != nil {
		return nil, fmt.Errorf("seal document: %w", err)
	}
	return out.Bytes(), nil
}

// Fingerprint is the SHA-256 of the sealing certificate, for the audit trail.
func (s *Sealer) Fingerprint() string {
	sum := sha256.Sum256(s.cert.Raw)
	return hex.EncodeToString(sum[:])
}

// Hash returns the hex SHA-256 of data.
func Hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

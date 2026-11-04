package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP follows RFC 6238 with the settings every authenticator app supports:
// SHA-1, six digits and a 30 second step.
const (
	totpDigits = 6
	totpStep   = 30
	// totpSkew accepts the previous and next code for clock drift.
	totpSkew = 1
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return totpEncoding.EncodeToString(buf), nil
}

func totpCode(secret string, counter uint64) (string, error) {
	key, err := totpEncoding.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, value%1_000_000), nil
}

// validTOTP checks a code against the secret at the given time.
func validTOTP(secret, code string, at time.Time) bool {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return false
	}
	counter := uint64(at.Unix() / totpStep)
	for delta := -totpSkew; delta <= totpSkew; delta++ {
		want, err := totpCode(secret, uint64(int64(counter)+int64(delta)))
		if err == nil && hmac.Equal([]byte(want), []byte(code)) {
			return true
		}
	}
	return false
}

// totpURI is what authenticator apps scan from the QR code.
func totpURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// sealSecret encrypts a value for storage with AES-GCM.
func sealSecret(key []byte, plain string) (string, error) {
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func openSecret(key []byte, sealed string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(deriveKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("sealed value is too short")
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	return string(plain), err
}

func deriveKey(key []byte) []byte {
	sum := sha256.Sum256(append([]byte("edms-secret-box:"), key...))
	return sum[:]
}

// newRecoveryCodes returns codes to show once, and their hashes to store.
func newRecoveryCodes(n int) (codes, hashes []string, err error) {
	for range n {
		code, err := randomCode(10, "abcdefghjkmnpqrstuvwxyz23456789")
		if err != nil {
			return nil, nil, err
		}
		code = code[:5] + "-" + code[5:]
		codes = append(codes, code)
		hashes = append(hashes, secretHash(code))
	}
	return codes, hashes, nil
}

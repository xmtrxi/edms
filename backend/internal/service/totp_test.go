package service

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1, truncated to six digits.
func TestTOTPMatchesRFCVectors(t *testing.T) {
	secret := totpEncoding.EncodeToString([]byte("12345678901234567890"))
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, tc := range cases {
		got, err := totpCode(secret, uint64(tc.unix/totpStep))
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("at %d: got %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestValidTOTPAcceptsAdjacentStepsOnly(t *testing.T) {
	secret, err := newTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	code := func(at time.Time) string {
		c, _ := totpCode(secret, uint64(at.Unix()/totpStep))
		return c
	}
	if !validTOTP(secret, code(now), now) {
		t.Fatal("current code rejected")
	}
	if !validTOTP(secret, code(now.Add(-30*time.Second)), now) {
		t.Fatal("previous code rejected")
	}
	if validTOTP(secret, code(now.Add(-90*time.Second)), now) {
		t.Fatal("stale code accepted")
	}
	if validTOTP(secret, "12345", now) || validTOTP(secret, "", now) {
		t.Fatal("malformed code accepted")
	}
}

func TestSealSecretRoundTrip(t *testing.T) {
	sealed, err := sealSecret([]byte("key"), "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "JBSWY3DPEHPK3PXP") {
		t.Fatal("secret stored in the clear")
	}
	plain, err := openSecret([]byte("key"), sealed)
	if err != nil || plain != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("got %q, %v", plain, err)
	}
	if _, err := openSecret([]byte("other"), sealed); err == nil {
		t.Fatal("opened with the wrong key")
	}
}

func TestRecoveryCodesAreUniqueAndHashed(t *testing.T) {
	codes, hashes, err := newRecoveryCodes(10)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i, code := range codes {
		if len(code) != 11 || code[5] != '-' {
			t.Fatalf("unexpected code shape %q", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
		if !secretMatches(hashes[i], code) {
			t.Fatalf("hash %d does not match its code", i)
		}
	}
}

func TestTOTPURI(t *testing.T) {
	uri := totpURI("EDMS", "ada@example.com", "ABC")
	if !strings.HasPrefix(uri, "otpauth://totp/EDMS:ada@example.com?") || !strings.Contains(uri, "secret=ABC") {
		t.Fatalf("unexpected uri %s", uri)
	}
}

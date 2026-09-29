package service

import "testing"

func TestVisitorSecrets(t *testing.T) {
	value := "A7K9P2Q4"
	hash := secretHash(value)
	if hash == value || !secretMatches(hash, value) || secretMatches(hash, "wrong") {
		t.Fatal("visitor secret hashing does not compare safely")
	}
}

func TestRandomCode(t *testing.T) {
	code, err := randomCode(8, "ABC123")
	if err != nil || len(code) != 8 {
		t.Fatalf("randomCode() = %q, %v", code, err)
	}
	for _, char := range code {
		if char != 'A' && char != 'B' && char != 'C' && char != '1' && char != '2' && char != '3' {
			t.Fatalf("unexpected character %q", char)
		}
	}
}

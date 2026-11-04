package service

import "testing"

func TestDetectSensitiveData(t *testing.T) {
	text := `Invoice for Jane (jane@acme.co.ke, +254712345678). KRA PIN A123456789Z.
National ID No: 12345678. Card 4111 1111 1111 1111 expires 09/28. Not a card: 1234 5678 9012 3456.`
	found := DetectSensitiveData(text)
	for kind, want := range map[string]int{"email": 1, "phone": 1, "kra_pin": 1, "national_id": 1, "card_number": 1} {
		if found[kind] != want {
			t.Errorf("%s: got %d, want %d (all: %v)", kind, found[kind], want, found)
		}
	}
}

func TestSuggestSensitivityOnlyRaises(t *testing.T) {
	strong := Findings{"card_number": 1}
	if got := SuggestSensitivity("internal", strong); got != "confidential" {
		t.Fatalf("card numbers should suggest confidential, got %q", got)
	}
	if got := SuggestSensitivity("restricted", strong); got != "" {
		t.Fatalf("never suggest lowering a label, got %q", got)
	}
	if got := SuggestSensitivity("internal", Findings{"email": 20}); got != "" {
		t.Fatalf("contact details alone keep the label, got %q", got)
	}
}

package service

import (
	"testing"

	"github.com/xmtrxi/edms/internal/database"
)

func TestAutomationConditions(t *testing.T) {
	doc := &database.Document{
		Name: "Supplier Contract 2026.PDF", Sensitivity: "",
		Metadata: map[string]any{
			"ai_classification": map[string]any{"category": "Contract", "confidence": 0.9},
			"detected_pii":      map[string]any{"email": 2, "kra_pin": uint64(1)},
		},
	}
	cases := []struct {
		name string
		cond database.AutomationCondition
		want bool
	}{
		{"empty matches everything", database.AutomationCondition{}, true},
		{"name is case-insensitive", database.AutomationCondition{NameContains: "contract"}, true},
		{"name mismatch", database.AutomationCondition{NameContains: "invoice"}, false},
		{"extension with dot and case", database.AutomationCondition{Extensions: []string{".pdf"}}, true},
		{"extension mismatch", database.AutomationCondition{Extensions: []string{"docx", "xlsx"}}, false},
		{"missing sensitivity is internal", database.AutomationCondition{Sensitivity: "internal"}, true},
		{"sensitivity mismatch", database.AutomationCondition{Sensitivity: "confidential"}, false},
		{"classification", database.AutomationCondition{Classification: "contract"}, true},
		{"classification mismatch", database.AutomationCondition{Classification: "invoice"}, false},
		{"strong identifiers", database.AutomationCondition{SensitiveData: true}, true},
		{"all together", database.AutomationCondition{NameContains: "supplier", Extensions: []string{"pdf"}, SensitiveData: true}, true},
	}
	for _, tc := range cases {
		if got := Matches(tc.cond, doc); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	plain := &database.Document{Name: "notes.txt", Metadata: map[string]any{"detected_pii": map[string]any{"email": 3}}}
	if Matches(database.AutomationCondition{SensitiveData: true}, plain) {
		t.Error("emails alone are not strong identifiers")
	}
}

package paperless

import "testing"

func TestDuplicateDocumentID(t *testing.T) {
	tests := []struct {
		name   string
		result string
		wantID int
		wantOK bool
	}{
		{
			name:   "paperless duplicate response",
			result: "universitycert.pdf: Not consuming universitycert.pdf: It is a duplicate of universitycert.pdf (#3).",
			wantID: 3,
			wantOK: true,
		},
		{
			name:   "case insensitive",
			result: "DUPLICATE document (#42)",
			wantID: 42,
			wantOK: true,
		},
		{
			name:   "ordinary failure",
			result: "Unable to consume malformed PDF",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := DuplicateDocumentID(tt.result)
			if id != tt.wantID || ok != tt.wantOK {
				t.Fatalf("DuplicateDocumentID(%q) = (%d, %v), want (%d, %v)", tt.result, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

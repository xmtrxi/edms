package queue

const (
	TaskOCR     = "document:ocr"
	TaskReindex = "search:reindex"
	TaskEmail   = "email:deliver"
)

type OCRPayload struct {
	DocumentID   string `json:"document_id"`
	StoragePath  string `json:"storage_path"`
	VersionID    string `json:"version_id,omitempty"`
	Checksum     string `json:"checksum,omitempty"`
	GenerationID string `json:"generation_id,omitempty"`
	JobID        string `json:"job_id,omitempty"`
}

type ReindexPayload struct{}

type EmailPayload struct {
	DeliveryID string `json:"delivery_id"`
}

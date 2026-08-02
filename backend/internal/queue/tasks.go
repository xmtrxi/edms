package queue

const (
	TaskOCR     = "document:ocr"
	TaskReindex = "search:reindex"
)

type OCRPayload struct {
	DocumentID  string `json:"document_id"`
	StoragePath string `json:"storage_path"`
}

type ReindexPayload struct{}

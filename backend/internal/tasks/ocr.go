package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"

	"github.com/xmtrxi/edms/internal/queue"
)

func (t Tasks) OCRDocument(ctx context.Context, task *asynq.Task) error {
	var payload queue.OCRPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	if err := t.ocr.ProcessOCR(ctx, payload.DocumentID, payload.StoragePath, payload.VersionID, payload.Checksum, payload.GenerationID, payload.JobID); err != nil {
		return err
	}
	// Labelling is advisory; a failure must not re-run OCR.
	if t.labels != nil {
		if err := t.labels.Process(ctx, payload.DocumentID); err != nil {
			t.log.Warn("label document", "document_id", payload.DocumentID, "error", err)
		}
	}
	return nil
}

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
	return t.ocr.ProcessOCR(ctx, payload.DocumentID, payload.StoragePath)
}

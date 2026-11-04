package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"

	"github.com/xmtrxi/edms/internal/queue"
)

// RunPDF processes a PDF tool job. The last retry marks the job failed so
// the person sees why instead of a job stuck in progress.
func (t Tasks) RunPDF(ctx context.Context, task *asynq.Task) error {
	var payload queue.PDFPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	retry, _ := asynq.GetRetryCount(ctx)
	max, _ := asynq.GetMaxRetry(ctx)
	return t.pdf.Process(ctx, payload.JobID, retry >= max)
}

package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/xmtrxi/edms/internal/queue"
)

func (t Tasks) DeliverEmail(ctx context.Context, task *asynq.Task) error {
	var payload queue.EmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	return t.email.DeliverEmail(ctx, payload.DeliveryID)
}

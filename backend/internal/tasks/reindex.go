package tasks

import (
	"context"

	"github.com/hibiken/asynq"
)

func (t Tasks) Reindex(ctx context.Context, task *asynq.Task) error {
	count, err := t.search.ReindexAll(ctx)
	if err != nil {
		return err
	}
	t.log.Info("search reindex complete", "indexed", count)
	return nil
}

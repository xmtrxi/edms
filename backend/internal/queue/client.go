package queue

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

type Client struct {
	inner *asynq.Client
}

func NewClient(addr string) (*Client, error) {
	opts, err := RedisOptions(addr)
	if err != nil {
		return nil, err
	}
	return &Client{inner: asynq.NewClient(opts)}, nil
}

func (c *Client) Enqueue(ctx context.Context, taskType string, payload any, opts ...asynq.Option) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = c.inner.EnqueueContext(ctx, asynq.NewTask(taskType, data, opts...))
	return err
}

func (c *Client) Close() error {
	return c.inner.Close()
}

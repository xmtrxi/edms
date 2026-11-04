package queue

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/hibiken/asynq"
)

// Inspector reads queue state. Tasks that use up their retries are kept as
// "archived", which is the queue's dead-letter list.
type Inspector struct {
	inner *asynq.Inspector
}

func NewInspector(addr string) (*Inspector, error) {
	opts, err := RedisOptions(addr)
	if err != nil {
		return nil, err
	}
	return &Inspector{inner: asynq.NewInspector(opts)}, nil
}

func (i *Inspector) Close() error { return i.inner.Close() }

// QueueStats summarises one queue.
type QueueStats struct {
	Queue     string `json:"queue"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Dead      int    `json:"dead"`
	Processed int    `json:"processed_today"`
	Failed    int    `json:"failed_today"`
	Paused    bool   `json:"paused"`
}

func (i *Inspector) Stats() ([]QueueStats, error) {
	names, err := i.inner.Queues()
	if err != nil {
		return nil, err
	}
	stats := make([]QueueStats, 0, len(names))
	for _, name := range names {
		info, err := i.inner.GetQueueInfo(name)
		if err != nil {
			return nil, err
		}
		stats = append(stats, QueueStats{
			Queue: name, Pending: info.Pending, Active: info.Active, Scheduled: info.Scheduled,
			Retry: info.Retry, Dead: info.Archived, Processed: info.Processed, Failed: info.Failed, Paused: info.Paused,
		})
	}
	return stats, nil
}

// DeadTask is a task that failed every retry.
type DeadTask struct {
	ID        string          `json:"id"`
	Queue     string          `json:"queue"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	LastError string          `json:"last_error"`
	Retried   int             `json:"retried"`
	FailedAt  time.Time       `json:"failed_at"`
}

// Dead lists dead tasks across every queue, newest failures first.
func (i *Inspector) Dead(limit int) ([]DeadTask, error) {
	names, err := i.inner.Queues()
	if err != nil {
		return nil, err
	}
	var tasks []DeadTask
	for _, name := range names {
		items, err := i.inner.ListArchivedTasks(name, asynq.PageSize(limit))
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			task := DeadTask{ID: item.ID, Queue: item.Queue, Type: item.Type, LastError: item.LastErr, Retried: item.Retried, FailedAt: item.LastFailedAt}
			// Payloads are small IDs; anything else is left out.
			if len(item.Payload) <= 2048 && json.Valid(item.Payload) {
				task.Payload = item.Payload
			}
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

// ErrTaskNotFound means the task is not in the dead-letter list.
var ErrTaskNotFound = errors.New("task not found")

// Requeue moves a dead task back to pending.
func (i *Inspector) Requeue(queue, id string) error {
	err := i.inner.RunTask(queue, id)
	if errors.Is(err, asynq.ErrTaskNotFound) || errors.Is(err, asynq.ErrQueueNotFound) {
		return ErrTaskNotFound
	}
	return err
}

// Discard deletes a dead task.
func (i *Inspector) Discard(queue, id string) error {
	err := i.inner.DeleteTask(queue, id)
	if errors.Is(err, asynq.ErrTaskNotFound) || errors.Is(err, asynq.ErrQueueNotFound) {
		return ErrTaskNotFound
	}
	return err
}

// RequeueAll moves every dead task in every queue back to pending.
func (i *Inspector) RequeueAll() (int, error) {
	names, err := i.inner.Queues()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, name := range names {
		n, err := i.inner.RunAllArchivedTasks(name)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

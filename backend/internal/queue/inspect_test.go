package queue

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// TestDeadLetterRoundTrip needs Valkey or Redis; set EDMS_TEST_VALKEY_URL.
func TestDeadLetterRoundTrip(t *testing.T) {
	url := os.Getenv("EDMS_TEST_VALKEY_URL")
	if url == "" {
		t.Skip("set EDMS_TEST_VALKEY_URL to run queue integration tests")
	}
	opts, err := RedisOptions(url)
	if err != nil {
		t.Fatal(err)
	}
	queueName := "test-" + time.Now().Format("150405.000000")
	client := asynq.NewClient(opts)
	defer func() { _ = client.Close() }()
	// MaxRetry(0) sends a failing task straight to the dead-letter list.
	info, err := client.Enqueue(asynq.NewTask("test:fail", []byte(`{"job_id":"x"}`)), asynq.Queue(queueName), asynq.MaxRetry(0))
	if err != nil {
		t.Fatal(err)
	}
	server := asynq.NewServer(opts, asynq.Config{Queues: map[string]int{queueName: 1}, Concurrency: 1, LogLevel: asynq.FatalLevel})
	mux := asynq.NewServeMux()
	mux.HandleFunc("test:fail", func(context.Context, *asynq.Task) error { return errors.New("always fails") })
	if err := server.Start(mux); err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown()

	inspector, err := NewInspector(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = inspector.Close() }()
	t.Cleanup(func() { _ = inspector.inner.DeleteQueue(queueName, true) })

	var dead []DeadTask
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		all, err := inspector.Dead(50)
		if err != nil {
			t.Fatal(err)
		}
		dead = dead[:0]
		for _, task := range all {
			if task.Queue == queueName {
				dead = append(dead, task)
			}
		}
		if len(dead) > 0 {
			break
		}
	}
	if len(dead) != 1 || dead[0].ID != info.ID || dead[0].LastError != "always fails" || string(dead[0].Payload) != `{"job_id":"x"}` {
		t.Fatalf("expected the failed task in the dead-letter list, got %+v", dead)
	}
	server.Shutdown()
	if err := inspector.Requeue(queueName, info.ID); err != nil {
		t.Fatalf("requeue: %v", err)
	}
	if err := inspector.Requeue(queueName, info.ID); err == nil {
		t.Fatal("a task that is already pending should not be requeued again")
	}
	if err := inspector.Discard(queueName, info.ID); err != nil {
		t.Fatalf("discard pending task: %v", err)
	}
	if err := inspector.Discard(queueName, info.ID); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("discarding twice should report not found, got %v", err)
	}
}

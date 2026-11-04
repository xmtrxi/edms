package service

import (
	"context"
	"log/slog"
	"time"
)

// Maintenance runs housekeeping on a schedule: it removes abandoned uploads
// and applies retention policies.
type Maintenance struct {
	uploads   *UploadService
	retention *RetentionService
	// claim returns true when this instance should do the work, so only one
	// of several API instances runs each pass.
	claim func(ctx context.Context, ttl time.Duration) bool
	log   *slog.Logger
	tasks []maintenanceTask
}

type maintenanceTask struct {
	name string
	run  func(context.Context) error
}

// Add registers another housekeeping step to run on every pass.
func (m *Maintenance) Add(name string, run func(context.Context) error) *Maintenance {
	m.tasks = append(m.tasks, maintenanceTask{name: name, run: run})
	return m
}

func NewMaintenance(uploads *UploadService, retention *RetentionService, claim func(context.Context, time.Duration) bool, logger *slog.Logger) *Maintenance {
	if logger == nil {
		logger = slog.Default()
	}
	if claim == nil {
		claim = func(context.Context, time.Duration) bool { return true }
	}
	return &Maintenance{uploads: uploads, retention: retention, claim: claim, log: logger}
}

// RunOnce does one pass.
func (m *Maintenance) RunOnce(ctx context.Context) {
	if removed, err := m.uploads.CleanupExpired(ctx); err != nil {
		m.log.Warn("clean up uploads", "error", err)
	} else if removed > 0 {
		m.log.Info("removed abandoned uploads", "count", removed)
	}
	if applied, err := m.retention.RunAll(ctx); err != nil {
		m.log.Warn("apply retention", "error", err)
	} else if applied > 0 {
		m.log.Info("applied retention", "documents", applied)
	}
	for _, task := range m.tasks {
		if err := task.run(ctx); err != nil {
			m.log.Warn("maintenance task failed", "task", task.name, "error", err)
		}
	}
}

// Start runs a pass shortly after startup and then every interval until ctx
// ends.
func (m *Maintenance) Start(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if m.claim(ctx, interval-time.Minute) {
				m.RunOnce(ctx)
			}
			timer.Reset(interval)
		}
	}
}

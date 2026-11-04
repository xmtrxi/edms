package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type ProcessingJobResponse struct {
	ID          string         `json:"id"`
	DocumentID  string         `json:"document_id"`
	Type        string         `json:"type"`
	Status      string         `json:"status"`
	Attempts    int            `json:"attempts"`
	LastError   string         `json:"last_error,omitempty"`
	ExternalID  string         `json:"external_id,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

func NewProcessingJobResponse(job *database.ProcessingJob) ProcessingJobResponse {
	return ProcessingJobResponse{ID: string(job.ID), DocumentID: job.DocumentID, Type: job.Type, Status: job.Status, Attempts: job.Attempts, LastError: job.LastError, ExternalID: job.ExternalID, Result: job.Result, CreatedAt: job.CreatedAt, UpdatedAt: job.UpdatedAt, CompletedAt: job.CompletedAt}
}

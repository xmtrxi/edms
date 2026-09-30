package service

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/queue"
)

type OperationsService struct {
	db    *surrealdb.DB
	smtp  config.SMTPConfig
	queue *queue.Client
}

func NewOperationsService(db *surrealdb.DB, smtpConfig config.SMTPConfig, queues ...*queue.Client) *OperationsService {
	var client *queue.Client
	if len(queues) > 0 {
		client = queues[0]
	}
	return &OperationsService{db: db, smtp: smtpConfig, queue: client}
}
func (s *OperationsService) Audit(ctx context.Context, action, resource string, since time.Time) ([]database.AuditEvent, error) {
	statement := "SELECT * FROM audit_events WHERE org_id = $org AND created_at >= $since"
	vars := map[string]any{"since": since, "org": orgOf(ctx)}
	if action != "" {
		statement += " AND action = $action"
		vars["action"] = action
	}
	if resource != "" {
		statement += " AND resource = $resource"
		vars["resource"] = resource
	}
	statement += " ORDER BY created_at DESC LIMIT 5000"
	return queryRows[database.AuditEvent](ctx, s.db, statement, vars)
}
func (s *OperationsService) PurgeAudit(ctx context.Context, before time.Time) error {
	if before.After(time.Now().UTC().AddDate(0, -1, 0)) {
		return fmt.Errorf("audit retention must keep at least 30 days")
	}
	_, err := surrealdb.Query[any](ctx, s.db, "DELETE audit_events WHERE org_id = $org AND created_at < $before", map[string]any{"before": before, "org": orgOf(ctx)})
	return err
}
func (s *OperationsService) Emails(ctx context.Context) ([]database.EmailDelivery, error) {
	return queryRows[database.EmailDelivery](ctx, s.db, "SELECT * FROM email_deliveries WHERE org_id = $org ORDER BY created_at DESC LIMIT 1000", map[string]any{"org": orgOf(ctx)})
}
func (s *OperationsService) QueueEmail(ctx context.Context, actor, recipient, subject, body string) (*database.EmailDelivery, error) {
	if !strings.Contains(recipient, "@") || strings.TrimSpace(subject) == "" || strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("recipient, subject and body are required")
	}
	now := time.Now().UTC()
	// A map, not the struct: a zero delivered_at would be sent as NULL, which
	// the optional datetime column rejects.
	item, err := surrealdb.Create[database.EmailDelivery](ctx, s.db, "email_deliveries", map[string]any{
		"org_id": orgOf(ctx), "recipient": strings.ToLower(strings.TrimSpace(recipient)), "subject": strings.TrimSpace(subject),
		"body": body, "status": "queued", "attempts": 0, "created_by": actor, "created_at": now, "updated_at": now,
	})
	if err != nil {
		return nil, err
	}
	return s.dispatch(ctx, item)
}
func (s *OperationsService) RetryEmail(ctx context.Context, id string) (*database.EmailDelivery, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, fmt.Errorf("email delivery not found")
	}
	item, err := surrealdb.Select[database.EmailDelivery](ctx, s.db, rid)
	if err != nil || item == nil {
		return nil, fmt.Errorf("email delivery not found")
	}
	if _, err = surrealdb.Merge[database.EmailDelivery](ctx, s.db, rid, map[string]any{"status": "queued", "updated_at": time.Now().UTC()}); err != nil {
		return nil, err
	}
	item.Status = "queued"
	return s.dispatch(ctx, item)
}
func (s *OperationsService) SetEmailStatus(ctx context.Context, id, status string) (*database.EmailDelivery, error) {
	if status != "bounced" && status != "delivered" && status != "failed" {
		return nil, fmt.Errorf("invalid delivery status")
	}
	updates := map[string]any{"status": status, "updated_at": time.Now().UTC()}
	if status == "delivered" {
		updates["delivered_at"] = time.Now().UTC()
	}
	return surrealdb.Merge[database.EmailDelivery](ctx, s.db, mustRecordID(id), updates)
}
func (s *OperationsService) dispatch(ctx context.Context, item *database.EmailDelivery) (*database.EmailDelivery, error) {
	if s.queue != nil {
		if err := s.queue.Enqueue(ctx, queue.TaskEmail, queue.EmailPayload{DeliveryID: string(item.ID)}); err != nil {
			return nil, err
		}
		return item, nil
	}
	return s.deliver(ctx, item)
}
func (s *OperationsService) DeliverEmail(ctx context.Context, id string) error {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return fmt.Errorf("email delivery not found")
	}
	item, err := surrealdb.Select[database.EmailDelivery](ctx, s.db, rid)
	if err != nil || item == nil {
		return fmt.Errorf("email delivery not found")
	}
	_, err = s.deliver(ctx, item)
	return err
}
func (s *OperationsService) deliver(ctx context.Context, item *database.EmailDelivery) (*database.EmailDelivery, error) {
	now := time.Now().UTC()
	attempts := item.Attempts + 1
	if s.smtp.Host == "" {
		return surrealdb.Merge[database.EmailDelivery](ctx, s.db, mustRecordID(string(item.ID)), map[string]any{"status": "failed", "attempts": attempts, "last_error": "SMTP is not configured", "updated_at": now})
	}
	auth := smtp.PlainAuth("", s.smtp.Username, s.smtp.Password, s.smtp.Host)
	message := []byte("To: " + item.Recipient + "\r\nSubject: " + item.Subject + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + item.Body)
	err := smtp.SendMail(s.smtp.Host+":"+s.smtp.Port, auth, s.smtp.From, []string{item.Recipient}, message)
	updates := map[string]any{"attempts": attempts, "updated_at": now}
	if err != nil {
		updates["status"] = "failed"
		updates["last_error"] = err.Error()
	} else {
		updates["status"] = "delivered"
		updates["last_error"] = ""
		updates["delivered_at"] = now
	}
	merged, mergeErr := surrealdb.Merge[database.EmailDelivery](ctx, s.db, mustRecordID(string(item.ID)), updates)
	if mergeErr != nil {
		return nil, mergeErr
	}
	return merged, nil
}

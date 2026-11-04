package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/observability"
)

var ErrNotificationNotFound = errors.New("notification not found")

// notifications delivers in-app notifications; nil turns notify into a
// no-op. It is set once at startup, like the realtime hub.
var notifications *NotificationService

// UseNotifications sets the service other services notify through.
func UseNotifications(n *NotificationService) { notifications = n }

// Notice is what to tell people. Link is an app path such as
// /documents/documents:abc.
type Notice struct {
	Kind       string
	Title      string
	Body       string
	Link       string
	Resource   string
	ResourceID string
	// Email also sends it by email to people who have email notifications on.
	Email bool
	// OrgID is needed when nobody is signed in, such as public uploads.
	OrgID string
}

// notify tells each recipient once, never the person who caused it.
func notify(ctx context.Context, recipients []string, notice Notice) {
	if notifications == nil {
		return
	}
	notifications.Send(context.WithoutCancel(ctx), recipients, notice)
}

// NotificationService stores notifications, pushes them to open tabs and
// emails people who asked for email.
type NotificationService struct {
	db    *surrealdb.DB
	email *OperationsService
	smtp  config.SMTPConfig
	log   *slog.Logger
}

func NewNotificationService(db *surrealdb.DB, email *OperationsService, smtp config.SMTPConfig, logger *slog.Logger) *NotificationService {
	if logger == nil {
		logger = slog.Default()
	}
	return &NotificationService{db: db, email: email, smtp: smtp, log: logger}
}

// Send records the notice for each recipient.
func (s *NotificationService) Send(ctx context.Context, recipients []string, notice Notice) {
	actor, _ := ActorFrom(ctx)
	seen := map[string]bool{}
	now := time.Now().UTC()
	for _, userID := range recipients {
		if userID == "" || seen[userID] || userID == actor.UserID || !strings.HasPrefix(userID, "users:") {
			continue
		}
		seen[userID] = true
		record := map[string]any{
			"user_id": userID, "kind": notice.Kind, "title": notice.Title, "body": notice.Body,
			"link": notice.Link, "actor_id": actor.UserID, "resource": notice.Resource,
			"resource_id": notice.ResourceID, "created_at": now,
		}
		if org := firstNonEmpty(notice.OrgID, orgOf(ctx)); org != "" {
			record["org_id"] = org
		}
		created, err := surrealdb.Create[database.Notification](ctx, s.db, "notifications", record)
		if err != nil {
			s.log.Warn("save notification", "user", userID, "kind", notice.Kind, "error", err)
			continue
		}
		observability.Notifications.WithLabelValues(notice.Kind).Inc()
		publishUser(ctx, userID, "notification", s.view(ctx, created))
		if notice.Email {
			s.sendEmail(ctx, userID, notice)
		}
	}
}

// routineKinds report progress rather than ask for action. People with the
// workflow digest on get them in one daily email instead of one at a time.
var routineKinds = []string{
	database.NotificationShare, database.NotificationRouteOutcome, database.NotificationFormOutcome,
	database.NotificationFileReceived, database.NotificationAccessDecision,
}

func (s *NotificationService) sendEmail(ctx context.Context, userID string, notice Notice) {
	if s.email == nil || s.smtp.Host == "" || !wantsEmail(ctx, s.db, userID) {
		return
	}
	if slices.Contains(routineKinds, notice.Kind) && userPreference(ctx, s.db, userID, "workflow_digest", true) {
		return
	}
	user, err := surrealdb.Select[database.User](ctx, s.db, mustRecordID(userID))
	if err != nil || user == nil || user.Email == "" || user.Status != database.UserStatusActive {
		return
	}
	body := notice.Title + "\n"
	if notice.Body != "" {
		body += "\n" + notice.Body + "\n"
	}
	if notice.Link != "" {
		body += "\nOpen it: " + strings.TrimRight(s.smtp.AppURL, "/") + notice.Link + "\n"
	}
	body += "\nYou can turn off these emails in Settings.\n"
	if _, err := s.email.QueueEmail(ctx, userID, user.Email, notice.Title, body); err != nil {
		s.log.Warn("email notification", "user", userID, "error", err)
	}
}

// wantsEmail reads the person's "Email notifications" preference, which is
// on unless they turned it off.
func wantsEmail(ctx context.Context, db *surrealdb.DB, userID string) bool {
	return userPreference(ctx, db, userID, "email_notifications", true)
}

func userPreference(ctx context.Context, db *surrealdb.DB, userID, key string, fallback bool) bool {
	rows, err := queryRows[database.UserSetting](ctx, db, "SELECT * FROM user_settings WHERE user_id = $user LIMIT 1", map[string]any{"user": userID})
	if err != nil || len(rows) == 0 {
		return fallback
	}
	if value, ok := rows[0].Values[key].(bool); ok {
		return value
	}
	return fallback
}

// NotificationView adds who caused the notification.
type NotificationView struct {
	database.Notification
	Actor *PersonView `json:"actor,omitempty"`
}

func (s *NotificationService) view(ctx context.Context, n *database.Notification) NotificationView {
	view := NotificationView{Notification: *n}
	if n.ActorID != "" {
		person := newPeople(s.db).get(ctx, n.ActorID)
		view.Actor = &person
	}
	return view
}

// NotificationPage is one page of a person's inbox.
type NotificationPage struct {
	Items  []NotificationView `json:"items"`
	Unread int                `json:"unread"`
}

// List returns the newest notifications in the current organization.
func (s *NotificationService) List(ctx context.Context, userID string, unreadOnly bool, limit int) (*NotificationPage, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	where := "user_id = $user AND (org_id = $org OR org_id = NONE)"
	vars := map[string]any{"user": userID, "org": orgOf(ctx), "limit": limit}
	statement := "SELECT * FROM notifications WHERE " + where
	if unreadOnly {
		statement += " AND read_at = NONE"
	}
	rows, err := queryRows[database.Notification](ctx, s.db, statement+" ORDER BY created_at DESC LIMIT $limit", vars)
	if err != nil {
		return nil, err
	}
	unread, err := queryCount(ctx, s.db, "SELECT count() AS count FROM notifications WHERE "+where+" AND read_at = NONE GROUP ALL", vars)
	if err != nil {
		return nil, err
	}
	people := newPeople(s.db)
	page := &NotificationPage{Items: make([]NotificationView, 0, len(rows)), Unread: unread}
	for _, n := range rows {
		view := NotificationView{Notification: n}
		if n.ActorID != "" {
			person := people.get(ctx, n.ActorID)
			view.Actor = &person
		}
		page.Items = append(page.Items, view)
	}
	return page, nil
}

// MarkRead marks one notification read.
func (s *NotificationService) MarkRead(ctx context.Context, userID, id string) error {
	rid, err := database.ParseRecordID(id)
	if err != nil || rid.Table != "notifications" {
		return ErrNotificationNotFound
	}
	rows, err := queryRows[database.Notification](ctx, s.db,
		"UPDATE $id SET read_at = time::now() WHERE user_id = $user AND read_at = NONE RETURN AFTER",
		map[string]any{"id": rid, "user": userID})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		existing, err := surrealdb.Select[database.Notification](ctx, s.db, rid)
		if err != nil || existing == nil || existing.UserID != userID {
			return ErrNotificationNotFound
		}
	}
	return nil
}

// MarkAllRead marks every notification in the current organization read.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	_, err := surrealdb.Query[any](ctx, s.db,
		"UPDATE notifications SET read_at = time::now() WHERE user_id = $user AND (org_id = $org OR org_id = NONE) AND read_at = NONE",
		map[string]any{"user": userID, "org": orgOf(ctx)})
	return err
}

// Clear deletes the person's read notifications in the current organization.
func (s *NotificationService) Clear(ctx context.Context, userID string) error {
	_, err := surrealdb.Query[any](ctx, s.db,
		"DELETE notifications WHERE user_id = $user AND (org_id = $org OR org_id = NONE) AND read_at != NONE",
		map[string]any{"user": userID, "org": orgOf(ctx)})
	return err
}

// Purge removes read notifications older than 90 days.
func (s *NotificationService) Purge(ctx context.Context) error {
	_, err := surrealdb.Query[any](ctx, s.db,
		"DELETE notifications WHERE read_at != NONE AND created_at < $cutoff",
		map[string]any{"cutoff": time.Now().UTC().Add(-90 * 24 * time.Hour)})
	return err
}

// SendDigests emails each person who wants the workflow digest a summary of
// routine updates they have not read, at most once a day.
func (s *NotificationService) SendDigests(ctx context.Context) error {
	if s.email == nil || s.smtp.Host == "" {
		return nil
	}
	type pending struct {
		UserID string `json:"user_id"`
	}
	week := time.Now().UTC().Add(-7 * 24 * time.Hour)
	users, err := queryRows[pending](ctx, s.db,
		"SELECT user_id FROM notifications WHERE read_at = NONE AND kind IN $kinds AND created_at > $since GROUP BY user_id",
		map[string]any{"kinds": routineKinds, "since": week})
	if err != nil {
		return err
	}
	for _, p := range users {
		if err := s.sendDigest(ctx, p.UserID, week); err != nil {
			s.log.Warn("send workflow digest", "user", p.UserID, "error", err)
		}
	}
	return nil
}

func (s *NotificationService) sendDigest(ctx context.Context, userID string, floor time.Time) error {
	if !userPreference(ctx, s.db, userID, "workflow_digest", true) || !wantsEmail(ctx, s.db, userID) {
		return nil
	}
	user, err := surrealdb.Select[database.User](ctx, s.db, mustRecordID(userID))
	if err != nil || user == nil || user.Status != database.UserStatusActive || user.Email == "" {
		return err
	}
	since := floor
	if user.LastDigestAt != nil {
		if time.Since(*user.LastDigestAt) < 23*time.Hour {
			return nil
		}
		if user.LastDigestAt.After(since) {
			since = *user.LastDigestAt
		}
	}
	items, err := queryRows[database.Notification](ctx, s.db,
		"SELECT * FROM notifications WHERE user_id = $user AND read_at = NONE AND kind IN $kinds AND created_at > $since ORDER BY created_at DESC LIMIT 25",
		map[string]any{"user": userID, "kinds": routineKinds, "since": since})
	if err != nil || len(items) == 0 {
		return err
	}
	base := strings.TrimRight(s.smtp.AppURL, "/")
	var body strings.Builder
	body.WriteString("Hello " + firstNonEmpty(user.Name, user.Email) + ",\n\nHere is what moved since your last summary:\n\n")
	for _, n := range items {
		body.WriteString("• " + n.Title + "\n")
		if n.Link != "" {
			body.WriteString("  " + base + n.Link + "\n")
		}
	}
	body.WriteString("\nSee everything in " + s.smtp.Product() + ": " + base + "/\n\nTurn off the daily summary in Settings to get these one at a time.\n")
	subject := fmt.Sprintf("Your %s summary: %d update", s.smtp.Product(), len(items))
	if len(items) != 1 {
		subject += "s"
	}
	if _, err := s.email.QueueEmail(ctx, userID, user.Email, subject, body.String()); err != nil {
		return err
	}
	_, err = surrealdb.Merge[database.User](ctx, s.db, mustRecordID(userID), map[string]any{"last_digest_at": time.Now().UTC()})
	return err
}

// departmentMemberIDs lists the people in a department, for steps assigned
// to a whole department.
func departmentMemberIDs(ctx context.Context, db *surrealdb.DB, departmentID string) []string {
	if departmentID == "" {
		return nil
	}
	members, err := queryRows[database.DepartmentMember](ctx, db, "SELECT * FROM department_members WHERE department_id = $department", map[string]any{"department": departmentID})
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(members))
	for _, m := range members {
		if !slices.Contains(ids, m.UserID) {
			ids = append(ids, m.UserID)
		}
	}
	return ids
}

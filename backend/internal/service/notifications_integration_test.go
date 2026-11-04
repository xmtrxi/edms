package service

import (
	"context"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

func TestNotificationInbox(t *testing.T) {
	auth := newTestAuth(t, nil)
	ctx := context.Background()
	ada, _ := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	actorCtx := WithActor(ctx, Actor{UserID: "users:someone-else", OrgID: ada.LastOrgID})

	svc := NewNotificationService(auth.DB, nil, config.SMTPConfig{}, nil)
	svc.Send(actorCtx, []string{string(ada.ID), string(ada.ID), "users:someone-else", "not-a-user"}, Notice{
		Kind: database.NotificationShare, Title: "Grace shared Budget.xlsx with you", Link: "/documents/documents:1",
	})
	svc.Send(actorCtx, []string{string(ada.ID)}, Notice{Kind: database.NotificationRouteStep, Title: "Approve the budget"})

	readerCtx := WithActor(ctx, Actor{UserID: string(ada.ID), OrgID: ada.LastOrgID})
	page, err := svc.List(readerCtx, string(ada.ID), false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || page.Unread != 2 {
		t.Fatalf("expected two unread notifications (duplicates and the actor skipped), got %d items, %d unread", len(page.Items), page.Unread)
	}
	// Stored times have one-second precision, so both may share a timestamp.
	var routeStep NotificationView
	for _, item := range page.Items {
		if item.Kind == database.NotificationRouteStep {
			routeStep = item
		}
	}
	if routeStep.ID == "" || routeStep.Actor == nil || routeStep.Actor.ID != "users:someone-else" {
		t.Fatalf("route step notification missing or without its actor: %+v", page.Items)
	}
	if err := svc.MarkRead(readerCtx, string(ada.ID), string(routeStep.ID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkRead(readerCtx, "users:intruder", string(routeStep.ID)); err == nil {
		t.Fatal("someone else marked the notification read")
	}
	page, _ = svc.List(readerCtx, string(ada.ID), true, 10)
	if len(page.Items) != 1 || page.Unread != 1 {
		t.Fatalf("expected one unread left, got %d / %d", len(page.Items), page.Unread)
	}
	if err := svc.Clear(readerCtx, string(ada.ID)); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkAllRead(readerCtx, string(ada.ID)); err != nil {
		t.Fatal(err)
	}
	page, _ = svc.List(readerCtx, string(ada.ID), false, 10)
	if len(page.Items) != 1 || page.Unread != 0 {
		t.Fatalf("clear should keep the then-unread item and mark-all should read it: %d / %d", len(page.Items), page.Unread)
	}
}

func TestWorkflowDigestBundlesRoutineUpdates(t *testing.T) {
	auth := newTestAuth(t, nil)
	ctx := context.Background()
	ada, _ := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Name: "Ada", Password: "correct horse"})
	ops := NewOperationsService(auth.DB, config.SMTPConfig{Host: "127.0.0.1", Port: "1", From: "edms@example.com", AppURL: "https://edms.example.com"})
	svc := NewNotificationService(auth.DB, ops, config.SMTPConfig{Host: "127.0.0.1", AppURL: "https://edms.example.com"}, nil)
	actorCtx := WithActor(ctx, Actor{UserID: "users:grace", OrgID: ada.LastOrgID})

	svc.Send(actorCtx, []string{string(ada.ID)}, Notice{Kind: database.NotificationFileReceived, Title: "Grace uploaded scan.pdf", Link: "/documents/documents:9", Email: true})
	svc.Send(actorCtx, []string{string(ada.ID)}, Notice{Kind: database.NotificationRouteStep, Title: "Approve the budget", Email: true})

	// The action-needed email goes out at once; the routine one waits for the digest.
	subjects := func() map[string]bool {
		rows, err := queryRows[database.EmailDelivery](ctx, auth.DB, "SELECT * FROM email_deliveries", nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, row := range rows {
			seen[row.Subject] = true
		}
		return seen
	}
	if got := subjects(); len(got) != 1 || !got["Approve the budget"] {
		t.Fatalf("expected only the action email, got %v", got)
	}
	if err := svc.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	if got := subjects(); len(got) != 2 || !got["Your Lumen DMS summary: 1 update"] {
		t.Fatalf("expected the digest email, got %v", got)
	}
	if err := svc.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	rows, _ := queryRows[database.EmailDelivery](ctx, auth.DB, "SELECT * FROM email_deliveries", nil)
	if len(rows) != 2 {
		t.Fatalf("digest sent twice within a day: %d emails", len(rows))
	}
	user, _ := surrealdb.Select[database.User](ctx, auth.DB, mustRecordID(string(ada.ID)))
	if user.LastDigestAt == nil || time.Since(*user.LastDigestAt) > time.Minute {
		t.Fatal("last digest time not recorded")
	}
}

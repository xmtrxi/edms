package service

import (
	"context"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/database"
)

func TestSearchRecentDocuments(t *testing.T) {
	auth := newTestAuth(t, nil)
	ctx := context.Background()
	ada, err := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, doc := range []struct {
		name    string
		changed time.Time
	}{
		{"old.pdf", now.AddDate(0, 0, -45)},
		{"last-week.pdf", now.AddDate(0, 0, -7)},
		{"today.pdf", now.Add(-time.Hour)},
	} {
		_, err := surrealdb.Create[database.Document](ctx, auth.DB, "documents", documentRecord(&database.Document{ID: database.RecordID(newDocumentID()), InheritPermissions: true,
			Name: doc.name, OriginalName: doc.name, OwnerID: string(ada.ID), OrgID: ada.LastOrgID,
			Status: "ready", Metadata: map[string]any{}, CreatedAt: doc.changed, UpdatedAt: doc.changed,
		}))
		if err != nil {
			t.Fatal(err)
		}
	}

	search := NewSearchService(auth.DB, nil, nil, nil)
	actorCtx := WithActor(ctx, Actor{UserID: string(ada.ID), OrgID: ada.LastOrgID})
	result, err := search.Search(actorCtx, SearchInput{
		OwnerID:      string(ada.ID),
		UpdatedSince: now.AddDate(0, 0, -30),
		Sort:         SortRecent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("expected the two documents changed in the last 30 days, got %d (total %d)", len(result.Items), result.Total)
	}
	if result.Items[0].Document.Name != "today.pdf" || result.Items[1].Document.Name != "last-week.pdf" {
		t.Fatalf("expected newest first, got %s then %s", result.Items[0].Document.Name, result.Items[1].Document.Name)
	}

	all, err := search.Search(actorCtx, SearchInput{OwnerID: string(ada.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 3 {
		t.Fatalf("without a date limit every document is listed, got %d", all.Total)
	}
}

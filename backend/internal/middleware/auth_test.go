package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/service"
	"github.com/xmtrxi/edms/internal/testdb"
)

func TestWebSocketToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/realtime", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Protocol", "edms.v1, the-token")
	if token, ok := websocketToken(req); !ok || token != "the-token" {
		t.Fatalf("got %q %v", token, ok)
	}
	req.Header.Set("Sec-WebSocket-Protocol", "other, the-token")
	if _, ok := websocketToken(req); ok {
		t.Fatal("unknown subprotocol accepted")
	}
	req.Header.Del("Upgrade")
	req.Header.Set("Sec-WebSocket-Protocol", "edms.v1, the-token")
	if _, ok := websocketToken(req); ok {
		t.Fatal("token read from a request that is not a WebSocket upgrade")
	}
}

func TestSafeMethod(t *testing.T) {
	for method, want := range map[string]bool{"GET": true, "HEAD": true, "OPTIONS": true, "PROPFIND": true, "POST": false, "PUT": false, "DELETE": false} {
		if safeMethod(method) != want {
			t.Errorf("safeMethod(%s) != %v", method, want)
		}
	}
}

// TestRequireAuth runs against SurrealDB; see internal/testdb.
func TestRequireAuth(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		Deployment: config.DeploymentConfig{Mode: config.ModeSelfHosted, Signup: config.SignupOpen},
		JWT:        config.JWTConfig{Secret: "test-secret-test-secret-test-secret", SessionTTL: time.Hour},
	}
	auth := service.NewAuthService(cfg, testdb.Open(t), nil)
	user, err := auth.Register(ctx, service.RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	login, err := auth.Login(ctx, service.LoginInput{Email: "ada@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	_, readOnly, err := auth.Tokens.Create(ctx, string(user.ID), user.LastOrgID, service.TokenInput{Name: "ci", Scopes: []string{service.ScopeRead}})
	if err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	e.Any("/thing", func(c *echo.Context) error {
		return c.String(http.StatusOK, CurrentUser(c).Email+"|"+CurrentSessionID(c))
	}, RequireAuth(auth))
	call := func(method, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/thing", nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := call(http.MethodGet, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", rec.Code)
	}
	if rec := call(http.MethodGet, "garbage"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", rec.Code)
	}
	rec := call(http.MethodGet, login.Token)
	if rec.Code != http.StatusOK || rec.Body.String() == "ada@example.com|" {
		t.Fatalf("session token: %d %q", rec.Code, rec.Body.String())
	}
	if rec := call(http.MethodGet, readOnly); rec.Code != http.StatusOK {
		t.Fatalf("read-only token on GET: %d", rec.Code)
	}
	if rec := call(http.MethodPost, readOnly); rec.Code != http.StatusForbidden {
		t.Fatalf("read-only token on POST: %d, want 403", rec.Code)
	}

	claims, _ := auth.ParseToken(login.Token)
	if err := auth.Sessions.Revoke(ctx, string(user.ID), claims.SessionID); err != nil {
		t.Fatal(err)
	}
	if rec := call(http.MethodGet, login.Token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session still accepted: %d", rec.Code)
	}
}

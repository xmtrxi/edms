package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/testdb"
)

func newTestAuth(t *testing.T, mutate func(*config.Config)) *AuthService {
	t.Helper()
	cfg := config.Config{
		App:        config.AppConfig{Env: "test"},
		Deployment: config.DeploymentConfig{Mode: config.ModeSelfHosted, Signup: config.SignupOpen, OrgName: "Test Office"},
		JWT:        config.JWTConfig{Secret: "test-secret-test-secret-test-secret", SessionTTL: time.Hour},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return NewAuthService(cfg, testdb.Open(t), nil)
}

func mustLogin(t *testing.T, auth *AuthService, email, password string) *LoginResult {
	t.Helper()
	result, err := auth.Login(context.Background(), LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatalf("login %s: %v", email, err)
	}
	return result
}

func TestSessionsCanBeSignedOut(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuth(t, nil)
	user, err := auth.Register(ctx, RegisterInput{Email: "Ada@Example.com", Name: "Ada", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	if user.EmailVerifiedAt == nil {
		t.Fatal("first self-hosted administrator should be trusted")
	}
	first := mustLogin(t, auth, "ada@example.com", "correct horse")
	second := mustLogin(t, auth, "ada@example.com", "correct horse")

	claims, _, err := auth.Authenticate(ctx, first.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	sessions, err := auth.Sessions.List(ctx, string(user.ID))
	if err != nil || len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d (%v)", len(sessions), err)
	}

	if err := auth.Sessions.Revoke(ctx, string(user.ID), claims.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.Authenticate(ctx, first.Token); err == nil {
		t.Fatal("signed-out token still works")
	}
	if _, _, err := auth.Authenticate(ctx, second.Token); err != nil {
		t.Fatalf("other session should survive: %v", err)
	}

	if _, err := auth.Login(ctx, LoginInput{Email: "ada@example.com", Password: "wrong password"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	if _, err := auth.Login(ctx, LoginInput{Email: "nobody@example.com", Password: "whatever1"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials for unknown address, got %v", err)
	}
}

func TestSwitchOrganizationKeepsTheSession(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuth(t, nil)
	user, err := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	login := mustLogin(t, auth, "ada@example.com", "correct horse")
	claims, _, _ := auth.Authenticate(ctx, login.Token)
	switched, _, err := auth.SwitchOrganization(ctx, user, claims.SessionID, user.LastOrgID)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := auth.Authenticate(ctx, switched)
	if err != nil || next.SessionID != claims.SessionID {
		t.Fatalf("switch should reuse session %s, got %+v (%v)", claims.SessionID, next, err)
	}
	if _, _, err := auth.SwitchOrganization(ctx, user, "", user.LastOrgID); err == nil {
		t.Fatal("switching without a session should fail")
	}
}

func TestPasswordResetIsSingleUseAndSignsOutEverywhere(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuth(t, nil)
	user, err := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	login := mustLogin(t, auth, "ada@example.com", "correct horse")

	// Unknown addresses succeed silently.
	if err := auth.RequestPasswordReset(ctx, "nobody@example.com"); err != nil {
		t.Fatal(err)
	}
	stale, err := auth.issueAuthToken(ctx, user, "password_reset", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.issueAuthToken(ctx, user, "password_reset", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := auth.ResetPassword(ctx, stale, "brand new pass"); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("an older link should stop working once a new one is sent, got %v", err)
	}
	if err := auth.ResetPassword(ctx, token, "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("expected weak password, got %v", err)
	}
	if err := auth.ResetPassword(ctx, token, "brand new pass"); err != nil {
		t.Fatal(err)
	}
	if err := auth.ResetPassword(ctx, token, "another new pass"); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("reset link reused: %v", err)
	}
	if _, _, err := auth.Authenticate(ctx, login.Token); err == nil {
		t.Fatal("reset should sign out existing sessions")
	}
	if _, err := auth.Login(ctx, LoginInput{Email: "ada@example.com", Password: "correct horse"}); err == nil {
		t.Fatal("old password still works")
	}
	mustLogin(t, auth, "ada@example.com", "brand new pass")

	expired, _ := auth.issueAuthToken(ctx, user, "password_reset", -time.Minute)
	if err := auth.ResetPassword(ctx, expired, "brand new pass 2"); !errors.Is(err, ErrInvalidLink) {
		t.Fatalf("expired link accepted: %v", err)
	}
}

func TestChangePasswordKeepsOnlyTheCurrentSession(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuth(t, nil)
	user, _ := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	here := mustLogin(t, auth, "ada@example.com", "correct horse")
	elsewhere := mustLogin(t, auth, "ada@example.com", "correct horse")
	claims, _, _ := auth.Authenticate(ctx, here.Token)

	if err := auth.ChangePassword(ctx, user, claims.SessionID, "wrong", "brand new pass"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected wrong current password to fail, got %v", err)
	}
	if err := auth.ChangePassword(ctx, user, claims.SessionID, "correct horse", "brand new pass"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := auth.Authenticate(ctx, here.Token); err != nil {
		t.Fatalf("current session should survive: %v", err)
	}
	if _, _, err := auth.Authenticate(ctx, elsewhere.Token); err == nil {
		t.Fatal("other sessions should be signed out")
	}
}

func TestEmailVerificationGatesSignIn(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuth(t, func(cfg *config.Config) {
		cfg.Deployment = config.DeploymentConfig{Mode: config.ModeSaaS, Signup: config.SignupOpen, PersonalWorkspaces: true, MultiOrg: true, RequireVerifiedEmail: true}
		cfg.SMTP = config.SMTPConfig{Host: "smtp.invalid", AppURL: "https://edms.example.com"}
	})
	user, err := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	if err != nil {
		t.Fatal(err)
	}
	if !auth.VerificationRequired(user) {
		t.Fatal("new SaaS account should need verification")
	}
	if _, err := auth.Login(ctx, LoginInput{Email: "ada@example.com", Password: "correct horse"}); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected unverified sign-in to be refused, got %v", err)
	}
	token, err := auth.issueAuthToken(ctx, user, "verify_email", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := auth.VerifyEmail(ctx, token)
	if err != nil || verified.EmailVerifiedAt == nil {
		t.Fatalf("verify: %v", err)
	}
	mustLogin(t, auth, "ada@example.com", "correct horse")
}

func TestTwoStepSignIn(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuth(t, nil)
	user, _ := auth.Register(ctx, RegisterInput{Email: "ada@example.com", Password: "correct horse"})
	login := mustLogin(t, auth, "ada@example.com", "correct horse")
	claims, _, _ := auth.Authenticate(ctx, login.Token)

	setup, err := auth.BeginMFA(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	code := func() string {
		c, _ := totpCode(setup.Secret, uint64(time.Now().Unix()/totpStep))
		return c
	}
	user, _ = auth.UserByID(ctx, string(user.ID))
	if _, err := auth.EnableMFA(ctx, user, claims.SessionID, "000000"); !errors.Is(err, ErrInvalidMFACode) && code() != "000000" {
		t.Fatalf("expected a wrong code to fail, got %v", err)
	}
	recovery, err := auth.EnableMFA(ctx, user, claims.SessionID, code())
	if err != nil || len(recovery) != 10 {
		t.Fatalf("enable: %v (%d codes)", err, len(recovery))
	}

	result := mustLogin(t, auth, "ada@example.com", "correct horse")
	if result.Token != "" || result.MFAToken == "" {
		t.Fatal("password alone should not start a session once two-step sign-in is on")
	}
	if _, _, err := auth.Authenticate(ctx, result.MFAToken); err == nil {
		t.Fatal("the challenge token must not work as a session token")
	}
	if _, _, err := auth.CompleteMFA(ctx, result.MFAToken, "nope", SessionMeta{}); !errors.Is(err, ErrInvalidMFACode) {
		t.Fatalf("expected bad code to fail, got %v", err)
	}
	if _, token, err := auth.CompleteMFA(ctx, result.MFAToken, code(), SessionMeta{}); err != nil || token == "" {
		t.Fatalf("complete with app code: %v", err)
	}

	result = mustLogin(t, auth, "ada@example.com", "correct horse")
	if _, _, err := auth.CompleteMFA(ctx, result.MFAToken, recovery[0], SessionMeta{}); err != nil {
		t.Fatalf("recovery code: %v", err)
	}
	if _, _, err := auth.CompleteMFA(ctx, result.MFAToken, recovery[0], SessionMeta{}); !errors.Is(err, ErrInvalidMFACode) {
		t.Fatal("a recovery code worked twice")
	}

	user, _ = auth.UserByID(ctx, string(user.ID))
	if err := auth.DisableMFA(ctx, user, "wrong", code()); err == nil {
		t.Fatal("disable without the password should fail")
	}
	if err := auth.DisableMFA(ctx, user, "correct horse", code()); err != nil {
		t.Fatal(err)
	}
	if result := mustLogin(t, auth, "ada@example.com", "correct horse"); result.Token == "" {
		t.Fatal("password should be enough again")
	}
}

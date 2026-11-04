package config

import (
	"reflect"
	"testing"
	"time"
)

func TestCORSOrigins(t *testing.T) {
	t.Setenv("CORS_ORIGIN", " https://a.example.com/ ,https://b.example.com,, ")
	if got := loadCORSOrigins(); !reflect.DeepEqual(got, []string{"https://a.example.com", "https://b.example.com"}) {
		t.Fatalf("got %v", got)
	}
	t.Setenv("CORS_ORIGIN", "")
	t.Setenv("APP_URL", "https://edms.example.com/")
	got := loadCORSOrigins()
	if got[len(got)-1] != "https://edms.example.com" || got[0] != "http://localhost:3000" {
		t.Fatalf("APP_URL and local dev origins expected, got %v", got)
	}
}

func TestDeploymentDefaults(t *testing.T) {
	t.Setenv("EDMS_MODE", "saas")
	d := loadDeployment()
	if d.Signup != SignupOpen || !d.RequireVerifiedEmail || !d.MultiOrg {
		t.Fatalf("SaaS defaults wrong: %+v", d)
	}
	t.Setenv("EDMS_MODE", "anything-else")
	d = loadDeployment()
	if d.Mode != ModeSelfHosted || d.Signup != SignupInvite || d.RequireVerifiedEmail {
		t.Fatalf("self-hosted defaults wrong: %+v", d)
	}
	t.Setenv("EDMS_REQUIRE_VERIFIED_EMAIL", "true")
	if !loadDeployment().RequireVerifiedEmail {
		t.Fatal("explicit verification setting ignored")
	}
}

func TestLoadSecurityAndSSO(t *testing.T) {
	t.Setenv("SESSION_TTL", "8h")
	t.Setenv("BODY_LIMIT_MB", "2")
	t.Setenv("LOGIN_MAX_ATTEMPTS", "4")
	t.Setenv("OIDC_ISSUER", "https://login.example.com/tenant/v2.0/")
	t.Setenv("OIDC_CLIENT_ID", "client")
	t.Setenv("OIDC_REDIRECT_URL", "https://edms.example.com/api/auth/sso/callback")
	t.Setenv("OIDC_ALLOWED_DOMAINS", "Example.com, @partner.org")
	cfg := Load("does-not-exist.env")
	if cfg.JWT.SessionTTL != 8*time.Hour || cfg.Security.BodyLimit != 2<<20 || cfg.Security.LoginAttempts != 4 {
		t.Fatalf("security settings not read: %+v %+v", cfg.JWT, cfg.Security)
	}
	if !cfg.SSO.Enabled() || cfg.SSO.Issuer != "https://login.example.com/tenant/v2.0" {
		t.Fatalf("SSO not enabled from env: %+v", cfg.SSO)
	}
	if !reflect.DeepEqual(cfg.SSO.AllowedDomains, []string{"example.com", "@partner.org"}) {
		t.Fatalf("allowed domains %v", cfg.SSO.AllowedDomains)
	}
	if !cfg.SSO.AutoProvision || cfg.SSO.Enforce {
		t.Fatalf("SSO defaults wrong: %+v", cfg.SSO)
	}
}

func TestEnvHelpersFallBack(t *testing.T) {
	t.Setenv("X_INT", "nope")
	t.Setenv("X_BOOL", "maybe")
	t.Setenv("X_DUR", "soon")
	if getEnvInt("X_INT", 7) != 7 || getEnvBool("X_BOOL", true) != true || getEnvDuration("X_DUR", time.Second) != time.Second {
		t.Fatal("invalid values should fall back to defaults")
	}
}

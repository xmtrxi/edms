package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/surrealdb/surrealdb.go"
	"golang.org/x/oauth2"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

var (
	ErrSSODisabled     = errors.New("single sign-on is not configured")
	ErrSSOState        = errors.New("sign-in expired or was started in another browser; try again")
	ErrSSONotPermitted = errors.New("your account is not permitted to sign in here")
)

const (
	tokenTypeSSOState = "sso_state"
	ssoStateTTL       = 10 * time.Minute
)

// SSOService signs people in through an OpenID Connect provider using the
// authorization code flow with PKCE.
type SSOService struct {
	cfg  config.SSOConfig
	auth *AuthService

	mu       sync.Mutex
	provider *oidc.Provider
}

func NewSSOService(cfg config.Config, auth *AuthService) *SSOService {
	return &SSOService{cfg: cfg.SSO, auth: auth}
}

func (s *SSOService) Enabled() bool { return s.cfg.Enabled() }

// Enforced reports whether password sign-in is turned off.
func (s *SSOService) Enforced() bool { return s.cfg.Enabled() && s.cfg.Enforce }

func (s *SSOService) Label() string {
	if !s.cfg.Enabled() {
		return ""
	}
	return s.cfg.Label
}

// SSOStart is where to send the browser and the state to keep in a cookie.
type SSOStart struct {
	URL   string
	State string
}

// Start builds the provider's sign-in URL. The returned state is a signed,
// short-lived token holding the CSRF state, nonce and PKCE verifier; it goes
// in an HttpOnly cookie and comes back with the callback.
func (s *SSOService) Start(ctx context.Context, returnTo string) (*SSOStart, error) {
	oauth, err := s.oauth(ctx)
	if err != nil {
		return nil, err
	}
	state, nonce := randomToken(), randomToken()
	verifier := oauth2.GenerateVerifier()
	if !strings.HasPrefix(returnTo, "/") || strings.HasPrefix(returnTo, "//") {
		returnTo = "/"
	}
	cookie, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"typ": tokenTypeSSOState, "state": state, "nonce": nonce, "verifier": verifier, "return": returnTo,
		"exp": time.Now().Add(ssoStateTTL).Unix(),
	}).SignedString(s.auth.jwtSecret())
	if err != nil {
		return nil, err
	}
	url := oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	return &SSOStart{URL: url, State: cookie}, nil
}

// SSOResult is a completed sign-in.
type SSOResult struct {
	User     *database.User
	Token    string
	ReturnTo string
}

// Finish checks the callback against the state cookie, exchanges the code
// and signs the person in, creating their account when allowed.
func (s *SSOService) Finish(ctx context.Context, stateCookie, state, code string, meta SessionMeta) (*SSOResult, error) {
	oauth, err := s.oauth(ctx)
	if err != nil {
		return nil, err
	}
	claims, err := s.auth.parse(stateCookie, tokenTypeSSOState)
	if err != nil || state == "" || claims["state"] != state {
		return nil, ErrSSOState
	}
	nonce, _ := claims["nonce"].(string)
	verifier, _ := claims["verifier"].(string)
	returnTo, _ := claims["return"].(string)

	token, err := oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("provider returned no id_token")
	}
	idToken, err := s.provider.Verifier(&oidc.Config{ClientID: s.cfg.ClientID}).Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("verify id_token: %w", err)
	}
	if idToken.Nonce != nonce {
		return nil, ErrSSOState
	}
	var idClaims map[string]any
	if err := idToken.Claims(&idClaims); err != nil {
		return nil, err
	}
	email, name, err := s.identity(idClaims)
	if err != nil {
		return nil, err
	}

	user, err := s.findOrCreate(ctx, email, name)
	if err != nil {
		return nil, err
	}
	meta.Method = database.SessionMethodSSO
	sessionToken, err := s.auth.SignIn(ctx, user, meta)
	if err != nil {
		return nil, err
	}
	_ = writeAudit(ctx, s.auth.DB, string(user.ID), "auth.sso_login", "user", string(user.ID), map[string]any{"issuer": s.cfg.Issuer})
	return &SSOResult{User: user, Token: sessionToken, ReturnTo: returnTo}, nil
}

// identity reads and checks the email and name from the ID token.
func (s *SSOService) identity(claims map[string]any) (email, name string, err error) {
	email, _ = claims[s.cfg.EmailClaim].(string)
	if email == "" {
		email, _ = claims["email"].(string)
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") {
		return "", "", fmt.Errorf("the identity provider did not send an email address")
	}
	// Providers that send email_verified must have verified the address.
	if verified, ok := claims["email_verified"].(bool); ok && !verified {
		return "", "", ErrSSONotPermitted
	}
	if len(s.cfg.AllowedDomains) > 0 {
		domain := email[strings.LastIndex(email, "@")+1:]
		allowed := false
		for _, d := range s.cfg.AllowedDomains {
			if domain == strings.TrimPrefix(d, "@") {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", "", ErrSSONotPermitted
		}
	}
	name, _ = claims["name"].(string)
	return email, strings.TrimSpace(name), nil
}

func (s *SSOService) findOrCreate(ctx context.Context, email, name string) (*database.User, error) {
	orgs := s.auth.Orgs
	user, err := orgs.userByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		if user.Status != database.UserStatusActive || user.Kind == database.UserKindAgent {
			return nil, ErrSSONotPermitted
		}
		if user.EmailVerifiedAt == nil {
			now := time.Now().UTC()
			user, err = surrealdb.Merge[database.User](ctx, s.auth.DB, mustRecordID(string(user.ID)), map[string]any{"email_verified_at": now, "updated_at": now})
			if err != nil {
				return nil, err
			}
		}
		// An existing self-hosted account may have no membership yet.
		if !orgs.Capabilities().IsSaaS() {
			if _, err := orgs.JoinDefaultOrganization(ctx, user); err != nil {
				return nil, err
			}
		}
		return user, nil
	}
	caps := orgs.Capabilities()
	if !s.cfg.AutoProvision || (caps.IsSaaS() && !caps.OpenSignup()) {
		return nil, ErrSSONotPermitted
	}
	now := time.Now().UTC()
	user, err = surrealdb.Create[database.User](ctx, s.auth.DB, "users", &database.User{
		Email: email, Name: firstNonEmpty(name, email), Status: database.UserStatusActive,
		EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, err
	}
	if caps.IsSaaS() {
		_, err = orgs.AfterSignup(ctx, user, "")
	} else {
		_, err = orgs.JoinDefaultOrganization(ctx, user)
	}
	return user, err
}

// oauth discovers the provider on first use, so the API starts even while
// the provider is unreachable.
func (s *SSOService) oauth(ctx context.Context) (*oauth2.Config, error) {
	if !s.cfg.Enabled() {
		return nil, ErrSSODisabled
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider == nil {
		provider, err := oidc.NewProvider(ctx, s.cfg.Issuer)
		if err != nil {
			return nil, fmt.Errorf("discover identity provider: %w", err)
		}
		s.provider = provider
	}
	scopes := s.cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "email", "profile"}
	}
	return &oauth2.Config{
		ClientID: s.cfg.ClientID, ClientSecret: s.cfg.ClientSecret, RedirectURL: s.cfg.RedirectURL,
		Endpoint: s.provider.Endpoint(), Scopes: scopes,
	}, nil
}

func randomToken() string {
	buf := make([]byte, 24)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

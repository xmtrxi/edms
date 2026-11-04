package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/surrealdb/surrealdb.go"
	"golang.org/x/crypto/bcrypt"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already registered")
	ErrRegistrationClosed = errors.New("registration is closed; request an invitation")
	ErrEmailNotVerified   = errors.New("confirm your email address before signing in")
	ErrInvalidMFACode     = errors.New("that code is not valid")
	ErrWeakPassword       = errors.New("password must be between 8 and 72 characters")
)

type AuthService struct {
	cfg      config.Config
	DB       *surrealdb.DB
	log      *slog.Logger
	Orgs     *OrganizationService
	Tokens   *TokenService
	Sessions *SessionService
	// Mail sends account emails; without it links are logged in development.
	Mail *OperationsService
}

func NewAuthService(cfg config.Config, db *surrealdb.DB, logger *slog.Logger) *AuthService {
	orgs := NewOrganizationService(cfg, db, logger)
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthService{cfg: cfg, DB: db, log: logger, Orgs: orgs, Tokens: NewTokenService(db, orgs), Sessions: NewSessionService(db, cfg.JWT.SessionTTL)}
}

// TokenClaims identifies the user, the session and the organization the
// session acts in.
type TokenClaims struct {
	UserID    string
	OrgID     string
	SessionID string
}

const (
	tokenTypeSession = "session"
	// tokenTypeMFA is the short-lived proof that the password was right,
	// exchanged for a session once the second step succeeds.
	tokenTypeMFA = "mfa"
	mfaTokenTTL  = 5 * time.Minute
)

func (s *AuthService) jwtSecret() []byte {
	if s.cfg.JWT.Secret == "" {
		return []byte("edms-dev-secret-change-me")
	}
	return []byte(s.cfg.JWT.Secret)
}

// StartSession records a new sign-in and returns its bearer token.
func (s *AuthService) StartSession(ctx context.Context, user *database.User, orgID string, meta SessionMeta) (string, error) {
	session, err := s.Sessions.Create(ctx, string(user.ID), meta)
	if err != nil {
		return "", err
	}
	return s.signSession(user, orgID, session)
}

func (s *AuthService) signSession(user *database.User, orgID string, session *database.Session) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"org":   orgID,
		"sid":   string(session.ID),
		"typ":   tokenTypeSession,
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   session.ExpiresAt.Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret())
}

func (s *AuthService) parse(tokenString, wantType string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.jwtSecret(), nil
	})
	if err != nil || !token.Valid {
		if err == nil {
			err = jwt.ErrTokenInvalidClaims
		}
		return nil, err
	}
	if typ, _ := claims["typ"].(string); typ != wantType {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

// ParseToken checks a session token's signature and expiry. It does not
// check whether the session was revoked; Authenticate does.
func (s *AuthService) ParseToken(tokenString string) (TokenClaims, error) {
	claims, err := s.parse(tokenString, tokenTypeSession)
	if err != nil {
		return TokenClaims{}, err
	}
	sub, _ := claims["sub"].(string)
	org, _ := claims["org"].(string)
	sid, _ := claims["sid"].(string)
	if sub == "" || sid == "" {
		return TokenClaims{}, jwt.ErrTokenInvalidClaims
	}
	return TokenClaims{UserID: sub, OrgID: org, SessionID: sid}, nil
}

// Authenticate resolves a session token to its live session and active user.
func (s *AuthService) Authenticate(ctx context.Context, tokenString string) (TokenClaims, *database.User, error) {
	claims, err := s.ParseToken(tokenString)
	if err != nil {
		return TokenClaims{}, nil, ErrInvalidCredentials
	}
	if _, err := s.Sessions.Active(ctx, claims.SessionID, claims.UserID); err != nil {
		return TokenClaims{}, nil, ErrInvalidCredentials
	}
	user, err := s.UserByID(ctx, claims.UserID)
	if err != nil || user == nil || user.Status != database.UserStatusActive {
		return TokenClaims{}, nil, ErrInvalidCredentials
	}
	return claims, user, nil
}

// SwitchOrganization re-issues the current session's token scoped to another
// organization the user belongs to and remembers it as their default.
func (s *AuthService) SwitchOrganization(ctx context.Context, user *database.User, sessionID, orgID string) (string, *database.Organization, error) {
	if sessionID == "" {
		return "", nil, fmt.Errorf("sign in to switch organizations")
	}
	session, err := s.Sessions.Active(ctx, sessionID, string(user.ID))
	if err != nil {
		return "", nil, err
	}
	if _, err := s.Orgs.Membership(ctx, orgID, string(user.ID)); err != nil {
		return "", nil, err
	}
	org, err := s.Orgs.Get(ctx, orgID)
	if err != nil {
		return "", nil, err
	}
	if err := s.Orgs.SetLastOrganization(ctx, string(user.ID), orgID); err != nil {
		return "", nil, err
	}
	token, err := s.signSession(user, orgID, session)
	return token, org, err
}

func (s *AuthService) UserByID(ctx context.Context, id string) (*database.User, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, err
	}
	return surrealdb.Select[database.User](ctx, s.DB, rid)
}

// ListUsers returns the active members of one organization.
func (s *AuthService) ListUsers(ctx context.Context, orgID string) ([]database.User, error) {
	return s.Orgs.MemberUsers(ctx, orgID)
}

type RegisterInput struct {
	Email            string
	Name             string
	Password         string
	OrganizationName string
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*database.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return nil, fmt.Errorf("a valid email address is required")
	}
	if err := checkPassword(in.Password); err != nil {
		return nil, err
	}

	existing, err := s.Orgs.userByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}
	registered, err := surrealdb.Query[[]database.User](ctx, s.DB, "SELECT * FROM users LIMIT 1", nil)
	if err != nil {
		return nil, err
	}
	firstUser := registered == nil || len(*registered) == 0 || len((*registered)[0].Result) == 0
	caps := s.Orgs.Capabilities()
	if !firstUser && !caps.OpenSignup() {
		return nil, ErrRegistrationClosed
	}
	if firstUser && !caps.IsSaaS() && caps.AdminEmail() != "" && !strings.EqualFold(caps.AdminEmail(), in.Email) {
		return nil, ErrRegistrationClosed
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &database.User{
		Email:        in.Email,
		Name:         in.Name,
		PasswordHash: hash,
		Status:       database.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	// The first administrator of a self-hosted install may have no mail
	// server yet, so they are trusted.
	if firstUser && !caps.IsSaaS() {
		user.EmailVerifiedAt = &now
	}

	created, err := surrealdb.Create[database.User](ctx, s.DB, "users", user)
	if err != nil {
		return nil, err
	}
	var org *database.Organization
	if firstUser {
		org, err = s.Orgs.FirstRegistration(ctx, created, in.OrganizationName)
	} else {
		org, err = s.Orgs.AfterSignup(ctx, created, in.OrganizationName)
	}
	if err != nil {
		return nil, err
	}
	created.LastOrgID = string(org.ID)
	if err := s.Orgs.SetLastOrganization(ctx, string(created.ID), string(org.ID)); err != nil {
		return nil, err
	}
	if created.EmailVerifiedAt == nil {
		if err := s.SendVerification(ctx, created); err != nil {
			s.log.Warn("send verification email", "user", created.ID, "error", err)
		}
	}
	return created, nil
}

// VerificationRequired reports whether this user must confirm their address
// before signing in.
func (s *AuthService) VerificationRequired(user *database.User) bool {
	return user.EmailVerifiedAt == nil && s.cfg.Deployment.RequireVerifiedEmail && s.cfg.SMTP.Host != ""
}

// AcceptInvitation creates the account for someone invited by email and adds
// them to the organization. People who already have an account sign in and
// accept through OrganizationService.AcceptInvitation instead.
func (s *AuthService) AcceptInvitation(ctx context.Context, token, name, password string, meta SessionMeta) (*database.User, string, error) {
	if err := checkPassword(password); err != nil {
		return nil, "", err
	}
	invite, err := s.Orgs.pendingInvitation(ctx, token)
	if err != nil {
		return nil, "", err
	}
	if existing, err := s.Orgs.userByEmail(ctx, invite.Email); err != nil {
		return nil, "", err
	} else if existing != nil {
		return nil, "", fmt.Errorf("an account already exists for %s; sign in to accept", invite.Email)
	}
	pass, err := hashPassword(password)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(name) == "" {
		name = invite.Name
	}
	now := time.Now().UTC()
	// The invitation was emailed, so following it proves the address.
	user, err := surrealdb.Create[database.User](ctx, s.DB, "users", &database.User{Email: invite.Email, Name: strings.TrimSpace(name), PasswordHash: pass, Status: database.UserStatusActive, LastOrgID: invite.OrgID, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, "", err
	}
	if _, err := s.Orgs.acceptFor(ctx, invite, user); err != nil {
		return nil, "", err
	}
	meta.Method = database.SessionMethodInvite
	tokenString, err := s.StartSession(ctx, user, invite.OrgID, meta)
	return user, tokenString, err
}

type LoginInput struct {
	Email    string
	Password string
	Meta     SessionMeta
}

// LoginResult carries either a session token, or, when two-step sign-in is
// on, a short-lived MFAToken to exchange with a code.
type LoginResult struct {
	User     *database.User
	Token    string
	MFAToken string
}

// dummyHash keeps sign-in timing the same whether or not the address exists.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("edms-timing-equaliser"), bcrypt.DefaultCost)

func (s *AuthService) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	user, err := s.Orgs.userByEmail(ctx, in.Email)
	if err != nil {
		return nil, err
	}
	if user == nil || user.Status != database.UserStatusActive || user.Kind == database.UserKindAgent || user.PasswordHash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if s.VerificationRequired(user) {
		return nil, ErrEmailNotVerified
	}
	if user.MFAEnabled {
		token, err := s.signMFAChallenge(user)
		if err != nil {
			return nil, err
		}
		return &LoginResult{User: user, MFAToken: token}, nil
	}
	in.Meta.Method = database.SessionMethodPassword
	token, err := s.SignIn(ctx, user, in.Meta)
	if err != nil {
		return nil, err
	}
	return &LoginResult{User: user, Token: token}, nil
}

// SignIn starts a session in the user's default organization.
func (s *AuthService) SignIn(ctx context.Context, user *database.User, meta SessionMeta) (string, error) {
	org, _, err := s.Orgs.Resolve(ctx, user, "")
	if err != nil {
		return "", err
	}
	return s.StartSession(ctx, user, string(org.ID), meta)
}

func checkPassword(password string) error {
	if len(password) < 8 || len(password) > 72 {
		return ErrWeakPassword
	}
	return nil
}

func hashPassword(password string) (string, error) {
	if err := checkPassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

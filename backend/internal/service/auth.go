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
)

type AuthService struct {
	cfg  config.Config
	DB   *surrealdb.DB
	log  *slog.Logger
	Orgs *OrganizationService
}

func NewAuthService(cfg config.Config, db *surrealdb.DB, logger *slog.Logger) *AuthService {
	return &AuthService{cfg: cfg, DB: db, log: logger, Orgs: NewOrganizationService(cfg, db, logger)}
}

// TokenClaims identifies the user and the organization the session acts in.
type TokenClaims struct {
	UserID string
	OrgID  string
}

func (s *AuthService) jwtSecret() []byte {
	if s.cfg.JWT.Secret == "" {
		return []byte("edms-dev-secret-change-me")
	}
	return []byte(s.cfg.JWT.Secret)
}

func (s *AuthService) SignToken(user *database.User, orgID string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"org":   orgID,
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(72 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret())
}

func (s *AuthService) ParseToken(tokenString string) (TokenClaims, error) {
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
		return TokenClaims{}, err
	}
	sub, _ := claims["sub"].(string)
	org, _ := claims["org"].(string)
	return TokenClaims{UserID: sub, OrgID: org}, nil
}

// SwitchOrganization issues a token scoped to another organization the user
// belongs to and remembers it as their default.
func (s *AuthService) SwitchOrganization(ctx context.Context, user *database.User, orgID string) (string, *database.Organization, error) {
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
	token, err := s.SignToken(user, orgID)
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
	if in.Email == "" || len(in.Password) < 8 {
		return nil, fmt.Errorf("email and a password of at least 8 characters are required")
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

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &database.User{
		Email:        in.Email,
		Name:         in.Name,
		PasswordHash: string(hash),
		Status:       database.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
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
	return created, s.Orgs.SetLastOrganization(ctx, string(created.ID), string(org.ID))
}

// AcceptInvitation creates the account for someone invited by email and adds
// them to the organization. People who already have an account sign in and
// accept through OrganizationService.AcceptInvitation instead.
func (s *AuthService) AcceptInvitation(ctx context.Context, token, name, password string) (*database.User, string, error) {
	if len(password) < 8 {
		return nil, "", fmt.Errorf("password must be at least 8 characters")
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
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(name) == "" {
		name = invite.Name
	}
	now := time.Now().UTC()
	user, err := surrealdb.Create[database.User](ctx, s.DB, "users", &database.User{Email: invite.Email, Name: strings.TrimSpace(name), PasswordHash: string(pass), Status: database.UserStatusActive, LastOrgID: invite.OrgID, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, "", err
	}
	if _, err := s.Orgs.acceptFor(ctx, invite, user); err != nil {
		return nil, "", err
	}
	tokenString, err := s.SignToken(user, invite.OrgID)
	return user, tokenString, err
}

type LoginInput struct {
	Email    string
	Password string
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (*database.User, string, error) {
	user, err := s.Orgs.userByEmail(ctx, in.Email)
	if err != nil {
		return nil, "", err
	}
	if user == nil || user.Status != database.UserStatusActive {
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}
	org, _, err := s.Orgs.Resolve(ctx, user, "")
	if err != nil {
		return nil, "", err
	}

	token, err := s.SignToken(user, string(org.ID))
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	surrealgoorm "github.com/kemboi22/surreal-go-orm"
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
	cfg config.Config
	DB  *surrealdb.DB
	log *slog.Logger
}

func NewAuthService(cfg config.Config, db *surrealdb.DB, logger *slog.Logger) *AuthService {
	return &AuthService{cfg: cfg, DB: db, log: logger}
}

func (s *AuthService) jwtSecret() []byte {
	if s.cfg.JWT.Secret == "" {
		return []byte("edms-dev-secret-change-me")
	}
	return []byte(s.cfg.JWT.Secret)
}

func (s *AuthService) SignToken(user *database.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(72 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret())
}

func (s *AuthService) ParseToken(tokenString string) (string, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.jwtSecret(), nil
	})
	if err != nil || !token.Valid {
		return "", err
	}
	sub, _ := claims["sub"].(string)
	return sub, nil
}

func (s *AuthService) UserByID(ctx context.Context, id string) (*database.User, error) {
	rid, err := database.ParseRecordID(id)
	if err != nil {
		return nil, err
	}
	return surrealdb.Select[database.User](ctx, s.DB, rid)
}

func (s *AuthService) ListUsers(ctx context.Context) ([]database.User, error) {
	result, err := surrealdb.Query[[]database.User](ctx, s.DB, "SELECT * FROM users WHERE status = $status ORDER BY name", map[string]any{"status": database.UserStatusActive})
	if err != nil || result == nil || len(*result) == 0 {
		return nil, err
	}
	return (*result)[0].Result, nil
}

type RegisterInput struct {
	Email    string
	Name     string
	Password string
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*database.User, error) {
	if in.Email == "" || in.Password == "" {
		return nil, ErrInvalidCredentials
	}

	existing, err := surrealgoorm.Query[database.User](s.DB, "users").WhereEq("email", in.Email).First(ctx)
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
	if !firstUser {
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
	authz, err := NewAuthorizationService(s.DB)
	if err != nil {
		return nil, err
	}
	if err := authz.BootstrapAdmin(ctx, string(created.ID)); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *AuthService) AcceptInvitation(ctx context.Context, token, password string) (*database.User, error) {
	if token == "" || len(password) < 8 {
		return nil, ErrInvalidCredentials
	}
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	r, err := surrealdb.Query[[]database.Invitation](ctx, s.DB, "SELECT * FROM invitations WHERE token_hash = $hash AND status = 'pending' LIMIT 1", map[string]any{"hash": hash})
	if err != nil || r == nil || len(*r) == 0 || len((*r)[0].Result) == 0 {
		return nil, fmt.Errorf("invitation is invalid or expired")
	}
	invite := (*r)[0].Result[0]
	if time.Now().UTC().After(invite.ExpiresAt) {
		return nil, fmt.Errorf("invitation is expired")
	}
	pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	user, err := surrealdb.Create[database.User](ctx, s.DB, "users", &database.User{Email: invite.Email, Name: invite.Name, PasswordHash: string(pass), Status: database.UserStatusActive, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return nil, err
	}
	rid, _ := database.ParseRecordID(string(invite.ID))
	_, err = surrealdb.Merge[database.Invitation](ctx, s.DB, rid, map[string]any{"status": "accepted", "accepted_at": now})
	return user, err
}

type LoginInput struct {
	Email    string
	Password string
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (*database.User, string, error) {
	user, err := surrealgoorm.Query[database.User](s.DB, "users").WhereEq("email", in.Email).First(ctx)
	if err != nil {
		return nil, "", err
	}
	if user == nil {
		return nil, "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.SignToken(user)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

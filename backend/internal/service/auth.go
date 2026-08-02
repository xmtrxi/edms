package service

import (
	"context"
	"errors"
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
	return surrealdb.Select[database.User](ctx, s.DB, id)
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

	return surrealdb.Create[database.User](ctx, s.DB, "users", user)
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

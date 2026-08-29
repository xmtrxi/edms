package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
)

type DepartmentInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
type AdministrationService struct {
	db    *surrealdb.DB
	authz *AuthorizationService
}

func NewAdministrationService(db *surrealdb.DB, authz *AuthorizationService) *AdministrationService {
	return &AdministrationService{db: db, authz: authz}
}
func (s *AdministrationService) Departments(ctx context.Context) ([]database.Department, error) {
	r, e := surrealdb.Query[[]database.Department](ctx, s.db, "SELECT * FROM departments ORDER BY name", nil)
	if e != nil || r == nil || len(*r) == 0 {
		return nil, e
	}
	return (*r)[0].Result, nil
}
func (s *AdministrationService) CreateDepartment(ctx context.Context, actor string, in DepartmentInput) (*database.Department, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("department name is required")
	}
	now := time.Now().UTC()
	return surrealdb.Create[database.Department](ctx, s.db, "departments", &database.Department{Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), CreatedBy: actor, CreatedAt: now, UpdatedAt: now})
}
func (s *AdministrationService) Members(ctx context.Context, id string) ([]database.DepartmentMember, error) {
	r, e := surrealdb.Query[[]database.DepartmentMember](ctx, s.db, "SELECT * FROM department_members WHERE department_id = $department", map[string]any{"department": id})
	if e != nil || r == nil || len(*r) == 0 {
		return nil, e
	}
	return (*r)[0].Result, nil
}
func (s *AdministrationService) AddMember(ctx context.Context, did, uid string) (*database.DepartmentMember, error) {
	return surrealdb.Create[database.DepartmentMember](ctx, s.db, "department_members", &database.DepartmentMember{DepartmentID: did, UserID: uid, Role: "member", CreatedAt: time.Now().UTC()})
}
func (s *AdministrationService) Roles(ctx context.Context) ([]database.Role, error) {
	r, e := surrealdb.Query[[]database.Role](ctx, s.db, "SELECT * FROM roles ORDER BY name", nil)
	if e != nil || r == nil || len(*r) == 0 {
		return nil, e
	}
	return (*r)[0].Result, nil
}
func (s *AdministrationService) AssignRole(ctx context.Context, uid, role string) error {
	return s.authz.AssignRole(ctx, uid, role, OrganizationDomain)
}
func (s *AdministrationService) Invite(ctx context.Context, cfg config.SMTPConfig, actor, name, email string) (string, error) {
	if strings.TrimSpace(email) == "" {
		return "", fmt.Errorf("email is required")
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	now := time.Now().UTC()
	if _, e := surrealdb.Create[database.Invitation](ctx, s.db, "invitations", &database.Invitation{Email: email, Name: name, TokenHash: hash, Status: "pending", ExpiresAt: now.Add(72 * time.Hour), CreatedBy: actor, CreatedAt: now}); e != nil {
		return "", e
	}
	link := strings.TrimRight(cfg.AppURL, "/") + "/auth/setup?token=" + token
	if cfg.Host != "" && cfg.From != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		_ = smtp.SendMail(cfg.Host+":"+cfg.Port, auth, cfg.From, []string{email}, []byte("To: "+email+"\r\nSubject: Your EDMS invitation\r\n\r\nSet your password: "+link+"\r\n"))
	}
	return link, nil
}

package dto

import (
	"time"

	"github.com/xmtrxi/edms/internal/database"
)

type UserResponse struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
	// EmailVerified and MFAEnabled describe the account's sign-in security.
	EmailVerified bool      `json:"email_verified"`
	MFAEnabled    bool      `json:"mfa_enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func NewUserResponse(u *database.User) UserResponse {
	if u == nil {
		return UserResponse{}
	}
	return UserResponse{
		ID:            string(u.ID),
		Email:         u.Email,
		Name:          u.Name,
		Status:        u.Status,
		EmailVerified: u.EmailVerifiedAt != nil,
		MFAEnabled:    u.MFAEnabled,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

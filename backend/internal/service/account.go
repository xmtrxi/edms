package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/surrealdb/surrealdb.go"
	"golang.org/x/crypto/bcrypt"

	"github.com/xmtrxi/edms/internal/database"
)

var ErrInvalidLink = errors.New("this link is invalid or has expired")

const (
	passwordResetTTL = time.Hour
	verifyEmailTTL   = 48 * time.Hour
)

// RequestPasswordReset emails a reset link. It succeeds whether or not the
// address has an account, so it cannot be used to discover accounts.
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.Orgs.userByEmail(ctx, email)
	if err != nil || user == nil || user.Status != database.UserStatusActive || user.Kind == database.UserKindAgent {
		return err
	}
	token, err := s.issueAuthToken(ctx, user, database.AuthTokenPasswordReset, passwordResetTTL)
	if err != nil {
		return err
	}
	link := s.appURL("/auth/reset-password", token)
	return s.mail(ctx, user, "Reset your "+s.cfg.SMTP.Product()+" password",
		"Hello "+displayName(user)+",\n\nSomeone asked to reset the password for your "+s.cfg.SMTP.Product()+" account. "+
			"If it was you, open this link within an hour to choose a new password:\n\n"+link+
			"\n\nIf you did not ask for this, you can ignore this email; your password has not changed.\n", link)
}

// ResetPassword sets a new password from an emailed link and signs the user
// out everywhere.
func (s *AuthService) ResetPassword(ctx context.Context, token, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	record, user, err := s.redeemAuthToken(ctx, token, database.AuthTokenPasswordReset)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	updates := map[string]any{"password_hash": hash, "password_changed_at": now, "updated_at": now}
	// Receiving the link proves the address.
	if user.EmailVerifiedAt == nil && strings.EqualFold(record.Email, user.Email) {
		updates["email_verified_at"] = now
	}
	if _, err := surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), updates); err != nil {
		return err
	}
	if err := s.Sessions.RevokeAll(ctx, string(user.ID), ""); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.DB, string(user.ID), "auth.password_reset", "user", string(user.ID), nil)
	return nil
}

// SendVerification emails a link that confirms the user's address.
func (s *AuthService) SendVerification(ctx context.Context, user *database.User) error {
	if user.EmailVerifiedAt != nil {
		return nil
	}
	token, err := s.issueAuthToken(ctx, user, database.AuthTokenVerifyEmail, verifyEmailTTL)
	if err != nil {
		return err
	}
	link := s.appURL("/auth/verify-email", token)
	return s.mail(ctx, user, "Confirm your email address",
		"Hello "+displayName(user)+",\n\nConfirm the email address for your "+s.cfg.SMTP.Product()+" account by opening this link:\n\n"+link+
			"\n\nThe link works for 48 hours.\n", link)
}

// ResendVerification is the public "send it again" for people who cannot
// sign in yet. Like password resets, it never reveals whether an account exists.
func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	user, err := s.Orgs.userByEmail(ctx, email)
	if err != nil || user == nil || user.Status != database.UserStatusActive {
		return err
	}
	return s.SendVerification(ctx, user)
}

// VerifyEmail confirms the address the link was sent to.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) (*database.User, error) {
	record, user, err := s.redeemAuthToken(ctx, token, database.AuthTokenVerifyEmail)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(record.Email, user.Email) {
		return nil, ErrInvalidLink
	}
	now := time.Now().UTC()
	user, err = surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"email_verified_at": now, "updated_at": now})
	if err == nil {
		_ = writeAudit(ctx, s.DB, string(user.ID), "auth.email_verified", "user", string(user.ID), nil)
	}
	return user, err
}

// ChangePassword replaces the password after checking the current one, and
// signs out every other session.
func (s *AuthService) ChangePassword(ctx context.Context, user *database.User, sessionID, current, next string) error {
	if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(next)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"password_hash": hash, "password_changed_at": now, "updated_at": now}); err != nil {
		return err
	}
	if err := s.Sessions.RevokeAll(ctx, string(user.ID), sessionID); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.DB, string(user.ID), "auth.password_changed", "user", string(user.ID), nil)
	_ = s.mail(ctx, user, "Your "+s.cfg.SMTP.Product()+" password was changed",
		"Hello "+displayName(user)+",\n\nThe password for your "+s.cfg.SMTP.Product()+" account was just changed and your other sessions were signed out. "+
			"If this was not you, reset your password at "+s.appURL("/auth/forgot-password", "")+" and contact your administrator.\n", "")
	return nil
}

// UpdateProfile changes the user's display name.
func (s *AuthService) UpdateProfile(ctx context.Context, user *database.User, name string) (*database.User, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return nil, fmt.Errorf("name must be between 1 and 120 characters")
	}
	return surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"name": name, "updated_at": time.Now().UTC()})
}

// MFASetup is what the person scans into an authenticator app.
type MFASetup struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// BeginMFA stores a new secret, not yet active, and returns it for scanning.
func (s *AuthService) BeginMFA(ctx context.Context, user *database.User) (*MFASetup, error) {
	if user.MFAEnabled {
		return nil, fmt.Errorf("two-step sign-in is already on")
	}
	secret, err := newTOTPSecret()
	if err != nil {
		return nil, err
	}
	sealed, err := sealSecret(s.encryptionKey(), secret)
	if err != nil {
		return nil, err
	}
	if _, err := surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"mfa_secret": sealed, "mfa_enabled": false, "updated_at": time.Now().UTC()}); err != nil {
		return nil, err
	}
	return &MFASetup{Secret: secret, URI: totpURI(s.issuer(), user.Email, secret)}, nil
}

// EnableMFA turns two-step sign-in on once the person proves their app shows
// the right code, and returns one-time recovery codes.
func (s *AuthService) EnableMFA(ctx context.Context, user *database.User, sessionID, code string) ([]string, error) {
	if user.MFAEnabled {
		return nil, fmt.Errorf("two-step sign-in is already on")
	}
	if !s.checkTOTP(user, code) {
		return nil, ErrInvalidMFACode
	}
	codes, hashes, err := newRecoveryCodes(10)
	if err != nil {
		return nil, err
	}
	if _, err := surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"mfa_enabled": true, "mfa_recovery_codes": hashes, "updated_at": time.Now().UTC()}); err != nil {
		return nil, err
	}
	// Sessions started with only a password should not outlive this change.
	_ = s.Sessions.RevokeAll(ctx, string(user.ID), sessionID)
	_ = writeAudit(ctx, s.DB, string(user.ID), "auth.mfa_enabled", "user", string(user.ID), nil)
	return codes, nil
}

// DisableMFA needs both the password and a current or recovery code.
func (s *AuthService) DisableMFA(ctx context.Context, user *database.User, password, code string) error {
	if !user.MFAEnabled {
		return nil
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	if ok, err := s.checkSecondFactor(ctx, user, code); err != nil || !ok {
		return ErrInvalidMFACode
	}
	if _, err := surrealdb.Query[any](ctx, s.DB, "UPDATE $id SET mfa_enabled = false, mfa_secret = NONE, mfa_recovery_codes = NONE, updated_at = time::now()", map[string]any{"id": mustRecordID(string(user.ID))}); err != nil {
		return err
	}
	_ = writeAudit(ctx, s.DB, string(user.ID), "auth.mfa_disabled", "user", string(user.ID), nil)
	return nil
}

// RegenerateRecoveryCodes replaces the recovery codes after a current code.
func (s *AuthService) RegenerateRecoveryCodes(ctx context.Context, user *database.User, code string) ([]string, error) {
	if !user.MFAEnabled {
		return nil, fmt.Errorf("two-step sign-in is off")
	}
	if !s.checkTOTP(user, code) {
		return nil, ErrInvalidMFACode
	}
	codes, hashes, err := newRecoveryCodes(10)
	if err != nil {
		return nil, err
	}
	_, err = surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"mfa_recovery_codes": hashes, "updated_at": time.Now().UTC()})
	return codes, err
}

// CompleteMFA exchanges the token from Login and a code for a session.
func (s *AuthService) CompleteMFA(ctx context.Context, mfaToken, code string, meta SessionMeta) (*database.User, string, error) {
	claims, err := s.parse(mfaToken, tokenTypeMFA)
	if err != nil {
		return nil, "", ErrInvalidLink
	}
	sub, _ := claims["sub"].(string)
	user, err := s.UserByID(ctx, sub)
	if err != nil || user == nil || user.Status != database.UserStatusActive || !user.MFAEnabled {
		return nil, "", ErrInvalidCredentials
	}
	if ok, err := s.checkSecondFactor(ctx, user, code); err != nil || !ok {
		return nil, "", ErrInvalidMFACode
	}
	meta.Method = database.SessionMethodPassword
	token, err := s.SignIn(ctx, user, meta)
	return user, token, err
}

func (s *AuthService) signMFAChallenge(user *database.User) (string, error) {
	claims := jwt.MapClaims{
		"sub": user.ID,
		"typ": tokenTypeMFA,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(mfaTokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret())
}

func (s *AuthService) checkTOTP(user *database.User, code string) bool {
	if user.MFASecret == "" {
		return false
	}
	secret, err := openSecret(s.encryptionKey(), user.MFASecret)
	return err == nil && validTOTP(secret, code, time.Now())
}

// checkSecondFactor accepts an authenticator code or, once, a recovery code.
func (s *AuthService) checkSecondFactor(ctx context.Context, user *database.User, code string) (bool, error) {
	if s.checkTOTP(user, code) {
		return true, nil
	}
	code = strings.ToLower(strings.TrimSpace(code))
	for i, hash := range user.MFARecoveryCodes {
		if secretMatches(hash, code) {
			remaining := slices.Delete(slices.Clone(user.MFARecoveryCodes), i, i+1)
			_, err := surrealdb.Merge[database.User](ctx, s.DB, mustRecordID(string(user.ID)), map[string]any{"mfa_recovery_codes": remaining, "updated_at": time.Now().UTC()})
			if err == nil {
				_ = writeAudit(ctx, s.DB, string(user.ID), "auth.recovery_code_used", "user", string(user.ID), map[string]any{"remaining": len(remaining)})
			}
			return err == nil, err
		}
	}
	return false, nil
}

func (s *AuthService) issueAuthToken(ctx context.Context, user *database.User, purpose string, ttl time.Duration) (string, error) {
	token, err := newShareToken()
	if err != nil {
		return "", err
	}
	// Older links for the same purpose stop working.
	if _, err := surrealdb.Query[any](ctx, s.DB, "UPDATE auth_tokens SET used_at = time::now() WHERE user_id = $user AND purpose = $purpose AND used_at = NONE",
		map[string]any{"user": string(user.ID), "purpose": purpose}); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	_, err = surrealdb.Create[database.AuthToken](ctx, s.DB, "auth_tokens", map[string]any{
		"user_id": string(user.ID), "purpose": purpose, "email": user.Email, "token_hash": secretHash(token),
		"expires_at": now.Add(ttl), "created_at": now,
	})
	return token, err
}

// redeemAuthToken marks an emailed token used and returns its user.
func (s *AuthService) redeemAuthToken(ctx context.Context, token, purpose string) (*database.AuthToken, *database.User, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil, ErrInvalidLink
	}
	rows, err := queryRows[database.AuthToken](ctx, s.DB,
		"SELECT * FROM auth_tokens WHERE token_hash = $hash AND purpose = $purpose LIMIT 1",
		map[string]any{"hash": secretHash(token), "purpose": purpose})
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 || rows[0].UsedAt != nil || time.Now().After(rows[0].ExpiresAt) {
		return nil, nil, ErrInvalidLink
	}
	record := rows[0]
	// Claim the token atomically so a link cannot be used twice in a race.
	claimed, err := queryRows[database.AuthToken](ctx, s.DB,
		"UPDATE $id SET used_at = time::now() WHERE used_at = NONE RETURN AFTER",
		map[string]any{"id": mustRecordID(string(record.ID))})
	if err != nil {
		return nil, nil, err
	}
	if len(claimed) == 0 {
		return nil, nil, ErrInvalidLink
	}
	user, err := s.UserByID(ctx, record.UserID)
	if err != nil || user == nil || user.Status != database.UserStatusActive {
		return nil, nil, ErrInvalidLink
	}
	return &record, user, nil
}

// mail sends an account email. Without SMTP, development logs the link so
// the flow can still be tried; production never does.
func (s *AuthService) mail(ctx context.Context, user *database.User, subject, body, link string) error {
	if s.Mail != nil && s.cfg.SMTP.Host != "" {
		_, err := s.Mail.QueueEmail(ctx, string(user.ID), user.Email, subject, body)
		return err
	}
	if link != "" && s.cfg.App.Env != "production" && s.cfg.App.Env != "prod" {
		s.log.Info("account email (SMTP not configured)", "to", user.Email, "subject", subject, "link", link)
	}
	return nil
}

func (s *AuthService) appURL(path, token string) string {
	base := strings.TrimRight(s.cfg.SMTP.AppURL, "/")
	if base == "" {
		base = "http://localhost:3000"
	}
	if token == "" {
		return base + path
	}
	return base + path + "?token=" + url.QueryEscape(token)
}

func (s *AuthService) encryptionKey() []byte {
	if s.cfg.JWT.EncryptionKey != "" {
		return []byte(s.cfg.JWT.EncryptionKey)
	}
	return s.jwtSecret()
}

func (s *AuthService) issuer() string {
	if name := strings.TrimSpace(s.cfg.Deployment.OrgName); name != "" && !s.Orgs.Capabilities().IsSaaS() {
		return name
	}
	return s.cfg.SMTP.Product()
}

func displayName(user *database.User) string {
	return firstNonEmpty(user.Name, user.Email)
}

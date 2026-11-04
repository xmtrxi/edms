package dto

type AuthResponse struct {
	Token        string               `json:"token,omitempty"`
	User         UserResponse         `json:"user"`
	Organization OrganizationResponse `json:"organization"`
	// MFAToken is returned instead of Token when the account uses two-step
	// sign-in; it is exchanged with a code at /auth/login/mfa.
	MFARequired bool   `json:"mfa_required,omitempty"`
	MFAToken    string `json:"mfa_token,omitempty"`
	// VerificationRequired is set when a new account must confirm its email
	// address before it can sign in.
	VerificationRequired bool `json:"verification_required,omitempty"`
}

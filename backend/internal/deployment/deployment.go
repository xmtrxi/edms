// Package deployment answers "what is this installation allowed to do?".
// Services ask these questions instead of comparing the mode string, so the
// SaaS and self-hosted editions share one code path.
package deployment

import "github.com/xmtrxi/edms/internal/config"

type Capabilities struct {
	cfg config.DeploymentConfig
}

func New(cfg config.DeploymentConfig) Capabilities {
	return Capabilities{cfg: cfg}
}

func (c Capabilities) Mode() string { return c.cfg.Mode }

func (c Capabilities) IsSaaS() bool { return c.cfg.Mode == config.ModeSaaS }

// OpenSignup reports whether anyone may create an account without an invitation.
func (c Capabilities) OpenSignup() bool { return c.cfg.Signup == config.SignupOpen }

// PersonalWorkspaces reports whether every account gets a private workspace.
func (c Capabilities) PersonalWorkspaces() bool { return c.cfg.PersonalWorkspaces }

// CanCreateOrganization reports whether a signed-in user may create another
// organization. Self-hosted installs run one organization unless multi-org is on.
func (c Capabilities) CanCreateOrganization(existing int) bool {
	if c.cfg.MultiOrg {
		return true
	}
	return existing == 0
}

// DefaultOrganizationName is used when the first organization is created.
func (c Capabilities) DefaultOrganizationName() string {
	if c.cfg.OrgName != "" {
		return c.cfg.OrgName
	}
	return "My organization"
}

func (c Capabilities) AdminEmail() string { return c.cfg.AdminEmail }

// Meta is the public, unauthenticated description the frontend uses to show
// or hide signup, the organization switcher and organization creation.
type Meta struct {
	Mode               string `json:"mode"`
	OpenSignup         bool   `json:"open_signup"`
	PersonalWorkspaces bool   `json:"personal_workspaces"`
	MultiOrg           bool   `json:"multi_org"`
	OrganizationName   string `json:"organization_name,omitempty"`
}

func (c Capabilities) Meta() Meta {
	meta := Meta{
		Mode:               c.cfg.Mode,
		OpenSignup:         c.cfg.Signup == config.SignupOpen,
		PersonalWorkspaces: c.cfg.PersonalWorkspaces,
		MultiOrg:           c.cfg.MultiOrg,
	}
	if !c.IsSaaS() {
		meta.OrganizationName = c.cfg.OrgName
	}
	return meta
}

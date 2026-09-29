package deployment

import (
	"testing"

	"github.com/xmtrxi/edms/internal/config"
)

func TestSelfHostedAllowsOneOrganization(t *testing.T) {
	caps := New(config.DeploymentConfig{Mode: config.ModeSelfHosted, Signup: config.SignupInvite})
	if caps.IsSaaS() || caps.OpenSignup() || caps.PersonalWorkspaces() {
		t.Fatalf("self-hosted defaults should be closed: %+v", caps.Meta())
	}
	if !caps.CanCreateOrganization(0) {
		t.Fatal("the first organization must be creatable")
	}
	if caps.CanCreateOrganization(1) {
		t.Fatal("self-hosted without multi-org must not create a second organization")
	}
}

func TestSaaSAllowsManyOrganizations(t *testing.T) {
	caps := New(config.DeploymentConfig{Mode: config.ModeSaaS, Signup: config.SignupOpen, MultiOrg: true, PersonalWorkspaces: true})
	if !caps.IsSaaS() || !caps.OpenSignup() || !caps.PersonalWorkspaces() {
		t.Fatalf("saas should be open: %+v", caps.Meta())
	}
	if !caps.CanCreateOrganization(5) {
		t.Fatal("saas must allow additional organizations")
	}
}

func TestDefaultOrganizationName(t *testing.T) {
	if got := New(config.DeploymentConfig{OrgName: "Acme Ltd"}).DefaultOrganizationName(); got != "Acme Ltd" {
		t.Fatalf("got %q", got)
	}
	if got := New(config.DeploymentConfig{}).DefaultOrganizationName(); got == "" {
		t.Fatal("fallback name must not be empty")
	}
}

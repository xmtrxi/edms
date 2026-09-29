package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type OrganizationsHandler struct {
	auth *service.AuthService
	orgs *service.OrganizationService
}

func NewOrganizationsHandler(deps Deps) *OrganizationsHandler {
	auth := service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)
	return &OrganizationsHandler{auth: auth, orgs: auth.Orgs}
}

func (h *OrganizationsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("", middleware.RequireAuth(h.auth))
	g.GET("/orgs", h.list)
	g.POST("/orgs", h.create)

	g.GET("/org", h.current)
	g.GET("/org/members", h.members)

	admin := g.Group("/org", middleware.RequireOrgAdmin())
	admin.PATCH("", h.rename)
	admin.PATCH("/members/:id", h.updateMember)
	admin.DELETE("/members/:id", h.removeMember)
	admin.GET("/invitations", h.invitations)
	admin.POST("/invitations", h.invite)
	admin.DELETE("/invitations/:id", h.revokeInvitation)
}

func (h *OrganizationsHandler) list(c *echo.Context) error {
	orgs, err := h.orgs.ForUser(c.Request().Context(), string(middleware.CurrentUser(c).ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list organizations")
	}
	items := make([]dto.OrganizationResponse, 0, len(orgs))
	for i := range orgs {
		items = append(items, dto.NewOrganizationResponse(&orgs[i].Organization, orgs[i].Role))
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

// create makes a new organization and returns a token already switched to it.
func (h *OrganizationsHandler) create(c *echo.Context) error {
	var r struct {
		Name string `json:"name"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	ctx := c.Request().Context()
	user := middleware.CurrentUser(c)
	org, err := h.orgs.Create(ctx, user, r.Name)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrOrganizationLimit) {
			status = http.StatusForbidden
		}
		return respondError(c, status, err.Error())
	}
	token, _, err := h.auth.SwitchOrganization(ctx, user, string(org.ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to switch organization")
	}
	return respondJSON(c, http.StatusCreated, dto.Created(dto.SwitchOrganizationResponse{Token: token, Organization: dto.NewOrganizationResponse(org, "owner")}))
}

func (h *OrganizationsHandler) current(c *echo.Context) error {
	role := ""
	if membership := middleware.CurrentMembership(c); membership != nil {
		role = membership.Role
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewOrganizationResponse(middleware.CurrentOrg(c), role)))
}

func (h *OrganizationsHandler) rename(c *echo.Context) error {
	var r struct {
		Name string `json:"name"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	org, err := h.orgs.Rename(c.Request().Context(), middleware.CurrentOrgID(c), r.Name)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewOrganizationResponse(org, middleware.CurrentMembership(c).Role)))
}

func (h *OrganizationsHandler) members(c *echo.Context) error {
	members, err := h.orgs.Members(c.Request().Context(), middleware.CurrentOrgID(c))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list members")
	}
	items := make([]dto.MemberResponse, 0, len(members))
	for _, member := range members {
		items = append(items, dto.MemberResponse{
			ID: string(member.ID), UserID: member.UserID, Email: member.Email, Name: member.Name,
			Role: member.Role, Status: member.Status, JoinedAt: member.JoinedAt, InvitedBy: member.InvitedBy,
		})
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *OrganizationsHandler) updateMember(c *echo.Context) error {
	var r service.MemberUpdate
	if err := bind(c, &r); err != nil {
		return err
	}
	membership, err := h.orgs.UpdateMember(c.Request().Context(), middleware.CurrentOrgID(c), middleware.CurrentMembership(c), c.Param("id"), r)
	if err != nil {
		return respondMemberError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(membership))
}

func (h *OrganizationsHandler) removeMember(c *echo.Context) error {
	if err := h.orgs.RemoveMember(c.Request().Context(), middleware.CurrentOrgID(c), middleware.CurrentMembership(c), c.Param("id")); err != nil {
		return respondMemberError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func respondMemberError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrAccessDenied):
		return respondError(c, http.StatusForbidden, "only owners can change owners")
	case errors.Is(err, service.ErrNotMember):
		return respondError(c, http.StatusNotFound, "member not found")
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}

func (h *OrganizationsHandler) invitations(c *echo.Context) error {
	invites, err := h.orgs.Invitations(c.Request().Context(), middleware.CurrentOrgID(c))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list invitations")
	}
	items := make([]dto.InvitationResponse, 0, len(invites))
	for i := range invites {
		items = append(items, dto.NewInvitationResponse(&invites[i]))
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *OrganizationsHandler) invite(c *echo.Context) error {
	var r service.InviteInput
	if err := bind(c, &r); err != nil {
		return err
	}
	if r.Role == "owner" && middleware.CurrentMembership(c).Role != "owner" {
		return respondError(c, http.StatusForbidden, "only owners can invite owners")
	}
	link, err := h.orgs.Invite(c.Request().Context(), middleware.CurrentOrgID(c), string(middleware.CurrentUser(c).ID), r)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(map[string]string{"invite_link": link}))
}

func (h *OrganizationsHandler) revokeInvitation(c *echo.Context) error {
	if err := h.orgs.RevokeInvitation(c.Request().Context(), middleware.CurrentOrgID(c), c.Param("id")); err != nil {
		return respondError(c, http.StatusNotFound, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

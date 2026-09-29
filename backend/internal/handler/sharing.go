package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
	"github.com/xmtrxi/edms/internal/storage"
)

type SharingHandler struct {
	auth       *service.AuthService
	sharing    *service.SharingService
	principals *service.PrincipalService
	storage    storage.Provider
}

func NewSharingHandler(deps Deps) *SharingHandler {
	auth := service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)
	return &SharingHandler{
		auth:       auth,
		sharing:    service.NewSharingService(deps.Cfg, deps.Client(), deps.Logger),
		principals: service.NewPrincipalService(deps.Client(), auth.Orgs),
		storage:    deps.Storage,
	}
}

func (h *SharingHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("", middleware.RequireAuth(h.auth))
	g.GET("/principals", h.searchPrincipals)
	g.GET("/documents/:id/access", h.access)
	g.PATCH("/documents/:id/access", h.updateSettings)
	g.PUT("/documents/:id/grants", h.upsertGrant)
	g.DELETE("/documents/:id/grants/:grant_id", h.removeGrant)
	g.POST("/documents/:id/shares", h.shareWithPerson)
	g.PATCH("/documents/:id/shares/:share_id", h.updateShareRole)
	g.DELETE("/documents/:id/shares/:share_id", h.revoke)
	g.POST("/documents/:id/links", h.createLink)
	g.POST("/documents/:id/access-requests", h.requestAccess)
	g.GET("/access-requests", h.accessRequests)
	g.POST("/access-requests/:id/approve", h.approve)
	g.POST("/access-requests/:id/deny", h.deny)
	g.GET("/org/sharing-policy", h.policy)
	g.PUT("/org/sharing-policy", h.updatePolicy, middleware.RequireOrgAdmin())

	// Links are opened without an account; organization-only links read the
	// bearer token when one is sent.
	api.GET("/links/:token", h.openLink)
	api.GET("/links/:token/download", h.downloadLink)
}

func (h *SharingHandler) searchPrincipals(c *echo.Context) error {
	actor, _ := service.ActorFrom(c.Request().Context())
	options, err := h.principals.Search(c.Request().Context(), actor.OrgID, c.QueryParam("q"), actor.IsGuest())
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to search people and teams")
	}
	return respondJSON(c, http.StatusOK, dto.List(options, int64(len(options)), 1, len(options)))
}

func (h *SharingHandler) access(c *echo.Context) error {
	result, err := h.sharing.Access(c.Request().Context(), actor(c), c.Param("id"), h.principals)
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(result))
}

func (h *SharingHandler) updateSettings(c *echo.Context) error {
	var r struct {
		InheritPermissions *bool  `json:"inherit_permissions"`
		Sensitivity        string `json:"sensitivity"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	if _, err := h.sharing.UpdateSettings(c.Request().Context(), actor(c), c.Param("id"), r.InheritPermissions, r.Sensitivity); err != nil {
		return sharingError(c, err)
	}
	return h.access(c)
}

func (h *SharingHandler) upsertGrant(c *echo.Context) error {
	var r service.GrantInput
	if err := bind(c, &r); err != nil {
		return err
	}
	grant, err := h.sharing.UpsertGrant(c.Request().Context(), actor(c), c.Param("id"), r)
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(grant))
}

func (h *SharingHandler) removeGrant(c *echo.Context) error {
	if err := h.sharing.RemoveGrant(c.Request().Context(), actor(c), c.Param("id"), c.Param("grant_id")); err != nil {
		return sharingError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *SharingHandler) shareWithPerson(c *echo.Context) error {
	var r service.PersonShareInput
	if err := bind(c, &r); err != nil {
		return err
	}
	result, err := h.sharing.ShareWithPerson(c.Request().Context(), actor(c), c.Param("id"), r)
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(result))
}

func (h *SharingHandler) updateShareRole(c *echo.Context) error {
	var r struct {
		Role string `json:"role"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	if err := h.sharing.UpdateShareRole(c.Request().Context(), actor(c), c.Param("id"), c.Param("share_id"), r.Role); err != nil {
		return sharingError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *SharingHandler) revoke(c *echo.Context) error {
	if err := h.sharing.Revoke(c.Request().Context(), actor(c), c.Param("id"), c.Param("share_id")); err != nil {
		return sharingError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *SharingHandler) createLink(c *echo.Context) error {
	var r service.LinkInput
	if err := bind(c, &r); err != nil {
		return err
	}
	link, url, err := h.sharing.CreateLink(c.Request().Context(), actor(c), c.Param("id"), r)
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(map[string]any{"link": link, "url": url}))
}

func (h *SharingHandler) requestAccess(c *echo.Context) error {
	var r struct {
		Role    string `json:"role"`
		Message string `json:"message"`
	}
	if err := bind(c, &r); err != nil {
		return err
	}
	request, err := h.sharing.RequestAccess(c.Request().Context(), actor(c), c.Param("id"), r.Role, r.Message)
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusCreated, dto.Created(request))
}

func (h *SharingHandler) accessRequests(c *echo.Context) error {
	items, err := h.sharing.AccessRequests(c.Request().Context(), actor(c))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list access requests")
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

func (h *SharingHandler) approve(c *echo.Context) error {
	var r struct {
		Role string `json:"role"`
	}
	_ = c.Bind(&r)
	request, err := h.sharing.DecideAccessRequest(c.Request().Context(), actor(c), c.Param("id"), true, r.Role)
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(request))
}

func (h *SharingHandler) deny(c *echo.Context) error {
	request, err := h.sharing.DecideAccessRequest(c.Request().Context(), actor(c), c.Param("id"), false, "")
	if err != nil {
		return sharingError(c, err)
	}
	return respondJSON(c, http.StatusOK, dto.OK(request))
}

func (h *SharingHandler) policy(c *echo.Context) error {
	policy, err := h.sharing.Policy(c.Request().Context(), middleware.CurrentOrgID(c))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to load sharing policy")
	}
	return respondJSON(c, http.StatusOK, dto.OK(policy))
}

func (h *SharingHandler) updatePolicy(c *echo.Context) error {
	var r service.SharePolicy
	if err := bind(c, &r); err != nil {
		return err
	}
	policy, err := h.sharing.UpdatePolicy(c.Request().Context(), middleware.CurrentOrgID(c), r)
	if err != nil {
		return respondError(c, http.StatusBadRequest, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(policy))
}

// linkViewer is the signed-in user opening a link, when a token was sent.
func (h *SharingHandler) linkViewer(c *echo.Context) *database.User {
	header := c.Request().Header.Get(echo.HeaderAuthorization)
	if !strings.HasPrefix(header, "Bearer ") {
		return nil
	}
	claims, err := h.auth.ParseToken(strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		return nil
	}
	user, err := h.auth.UserByID(c.Request().Context(), claims.UserID)
	if err != nil || user == nil || user.Status != database.UserStatusActive {
		return nil
	}
	return user
}

type linkResponse struct {
	Document    dto.DocumentResponse `json:"document"`
	Role        string               `json:"role"`
	LinkScope   string               `json:"link_scope,omitempty"`
	CanDownload bool                 `json:"can_download"`
	ExpiresAt   string               `json:"expires_at"`
}

func (h *SharingHandler) openLink(c *echo.Context) error {
	opened, err := h.sharing.OpenLink(c.Request().Context(), c.Param("token"), c.Request().Header.Get("X-Link-Password"), h.linkViewer(c))
	if err != nil {
		return sharingError(c, err)
	}
	document := dto.NewDocumentResponse(opened.Document)
	// Link viewers get the file, not internal metadata such as storage paths.
	document.Metadata = nil
	return respondJSON(c, http.StatusOK, dto.OK(linkResponse{
		Document: document, Role: opened.Share.Permission, LinkScope: opened.Share.LinkScope,
		CanDownload: service.LinkAllows(opened.Share, "download"), ExpiresAt: opened.Share.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}))
}

// downloadLink streams the file. inline=1 serves it for the in-browser viewer.
func (h *SharingHandler) downloadLink(c *echo.Context) error {
	password := c.Request().Header.Get("X-Link-Password")
	if password == "" {
		password = c.QueryParam("password")
	}
	opened, err := h.sharing.OpenLink(c.Request().Context(), c.Param("token"), password, h.linkViewer(c))
	if err != nil {
		return sharingError(c, err)
	}
	inline := c.QueryParam("inline") == "1"
	if !inline && !service.LinkAllows(opened.Share, "download") {
		return respondError(c, http.StatusForbidden, "downloads are turned off for this link")
	}
	reader, err := h.storage.Download(c.Request().Context(), opened.Document.StoragePath)
	if err != nil {
		return respondError(c, http.StatusBadGateway, "file is not available")
	}
	defer reader.Close()
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, disposition+"; filename=\""+strings.ReplaceAll(opened.Document.Name, "\"", "")+"\"")
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return c.Stream(http.StatusOK, opened.Document.MimeType, reader)
}

func sharingError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound), errors.Is(err, service.ErrShareNotFound):
		return respondError(c, http.StatusNotFound, "not found")
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrAccessDenied):
		return respondError(c, http.StatusForbidden, "you do not have permission to do this")
	case errors.Is(err, service.ErrLinkPassword):
		return respondError(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, service.ErrSharingDisabled), errors.Is(err, service.ErrRestrictedSharing):
		return respondError(c, http.StatusForbidden, err.Error())
	default:
		return respondError(c, http.StatusBadRequest, err.Error())
	}
}

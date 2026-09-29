package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type AdministrationHandler struct {
	auth    *service.AuthService
	authz   *service.AuthorizationService
	service *service.AdministrationService
}

func NewAdministrationHandler(deps Deps) *AdministrationHandler {
	authz, err := service.NewAuthorizationService(deps.Client())
	if err != nil {
		panic(err)
	}
	auth := service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)
	return &AdministrationHandler{auth: auth, authz: authz, service: service.NewAdministrationService(deps.Client(), authz, auth.Orgs)}
}
func (h *AdministrationHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/admin", middleware.RequireAuth(h.auth))
	d := g.Group("/departments", middleware.RequirePermission(h.authz, "departments", "manage"))
	d.GET("", h.listDepartments)
	d.POST("", h.createDepartment)
	d.GET("/:id/members", h.members)
	d.POST("/:id/members", h.addMember)
	r := g.Group("/roles", middleware.RequirePermission(h.authz, "roles", "manage"))
	r.GET("", h.listRoles)
	r.POST("", h.createRole)
	r.PUT("/:key/permissions", h.setPermissions)
	r.PUT("/:key/users/:userID", h.assignRole)
	// Kept for older clients; new clients use POST /api/org/invitations.
	g.POST("/invitations", h.invite, middleware.RequirePermission(h.authz, "users", "manage"))
}
func (h *AdministrationHandler) invite(c *echo.Context) error {
	var r service.InviteInput
	if e := bind(c, &r); e != nil {
		return e
	}
	if membership := middleware.CurrentMembership(c); r.Role == "owner" && (membership == nil || membership.Role != "owner") {
		return respondError(c, http.StatusForbidden, "only owners can invite owners")
	}
	link, e := h.auth.Orgs.Invite(c.Request().Context(), middleware.CurrentOrgID(c), string(middleware.CurrentUser(c).ID), r)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(map[string]string{"setup_link": link}))
}
func (h *AdministrationHandler) listDepartments(c *echo.Context) error {
	items, e := h.service.Departments(c.Request().Context(), middleware.CurrentOrgID(c))
	if e != nil {
		return respondError(c, 500, "failed to list departments")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *AdministrationHandler) createDepartment(c *echo.Context) error {
	var r service.DepartmentInput
	if e := bind(c, &r); e != nil {
		return e
	}
	item, e := h.service.CreateDepartment(c.Request().Context(), middleware.CurrentOrgID(c), string(middleware.CurrentUser(c).ID), r)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *AdministrationHandler) members(c *echo.Context) error {
	items, e := h.service.Members(c.Request().Context(), middleware.CurrentOrgID(c), c.Param("id"))
	if e != nil {
		return respondError(c, 404, e.Error())
	}
	return respondJSON(c, 200, dto.OK(items))
}
func (h *AdministrationHandler) addMember(c *echo.Context) error {
	var r struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if e := bind(c, &r); e != nil {
		return e
	}
	item, e := h.service.AddMember(c.Request().Context(), middleware.CurrentOrgID(c), c.Param("id"), r.UserID, r.Role)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *AdministrationHandler) listRoles(c *echo.Context) error {
	items, e := h.service.Roles(c.Request().Context(), middleware.CurrentOrgID(c))
	if e != nil {
		return respondError(c, 500, "failed to list roles")
	}
	return respondJSON(c, 200, dto.List(items, int64(len(items)), 1, len(items)))
}
func (h *AdministrationHandler) createRole(c *echo.Context) error {
	var r struct {
		Key         string   `json:"key"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if e := bind(c, &r); e != nil {
		return e
	}
	item, e := h.authz.CreateRole(c.Request().Context(), middleware.CurrentOrgID(c), r.Key, r.Name, r.Description, r.Permissions)
	if e != nil {
		return respondError(c, 400, e.Error())
	}
	return respondJSON(c, http.StatusCreated, dto.Created(item))
}
func (h *AdministrationHandler) setPermissions(c *echo.Context) error {
	var r struct {
		Permissions []string `json:"permissions"`
	}
	if e := bind(c, &r); e != nil {
		return e
	}
	orgID := middleware.CurrentOrgID(c)
	if !h.roleInOrg(c, orgID, c.Param("key")) {
		return respondError(c, 404, "role not found")
	}
	if e := h.authz.ReplaceRolePermissions(c.Request().Context(), orgID, c.Param("key"), r.Permissions); e != nil {
		return respondError(c, 400, e.Error())
	}
	return c.NoContent(http.StatusNoContent)
}
func (h *AdministrationHandler) assignRole(c *echo.Context) error {
	if e := h.service.AssignRole(c.Request().Context(), middleware.CurrentOrgID(c), c.Param("userID"), c.Param("key")); e != nil {
		return respondError(c, 400, e.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *AdministrationHandler) roleInOrg(c *echo.Context, orgID, key string) bool {
	roles, err := h.service.Roles(c.Request().Context(), orgID)
	if err != nil {
		return false
	}
	for _, role := range roles {
		if role.Key == key && role.OrgID == orgID {
			return true
		}
	}
	return false
}

package handler

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/surrealdb/surrealdb.go"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type DepartmentsHandler struct {
	db   *surrealdb.DB
	auth *service.AuthService
}

func NewDepartmentsHandler(deps Deps) *DepartmentsHandler {
	return &DepartmentsHandler{db: deps.Client(), auth: service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger)}
}
func (h *DepartmentsHandler) RegisterRoutes(api *echo.Group) {
	api.GET("/departments", h.list, middleware.RequireAuth(h.auth))
}
func (h *DepartmentsHandler) list(c *echo.Context) error {
	result, err := surrealdb.Query[[]database.Department](c.Request().Context(), h.db, "SELECT * FROM departments ORDER BY name", nil)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list departments")
	}
	items := []database.Department{}
	if result != nil && len(*result) > 0 {
		items = (*result)[0].Result
	}
	return respondJSON(c, http.StatusOK, dto.List(items, int64(len(items)), 1, len(items)))
}

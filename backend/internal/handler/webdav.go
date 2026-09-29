package handler

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"golang.org/x/net/webdav"

	"github.com/xmtrxi/edms/internal/davfs"
	"github.com/xmtrxi/edms/internal/service"
)

// WebDAVPrefix is where the drive is mounted.
const WebDAVPrefix = "/webdav"

// webdavMethods are every method a WebDAV client may send.
var webdavMethods = []string{
	http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions,
	"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK",
}

// readOnlyMethods are allowed with a read-only token.
var readOnlyMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND"}

type WebDAVHandler struct {
	auth *service.AuthService
	dav  *webdav.Handler
}

func NewWebDAVHandler(deps Deps) *WebDAVHandler {
	db := deps.Client()
	docs := service.NewDocumentService(deps.Cfg, db, deps.Storage, deps.Queue, deps.Search, deps.Logger)
	fs := davfs.New(davfs.Services{
		Docs:      docs,
		Folders:   service.NewFolderService(db),
		Versions:  service.NewVersionService(db, deps.Storage, docs),
		Workspace: service.NewWorkspaceService(db, deps.Cfg, deps.Storage),
		Search:    service.NewSearchService(db, deps.Storage, deps.Search, deps.Logger),
		Access:    service.NewAccessService(db),
		Storage:   deps.Storage,
	})
	return &WebDAVHandler{
		auth: service.NewAuthService(deps.Cfg, db, deps.Logger),
		dav:  &webdav.Handler{Prefix: WebDAVPrefix, FileSystem: fs, LockSystem: webdav.NewMemLS()},
	}
}

// Register mounts the drive on the root router, outside /api.
func (h *WebDAVHandler) Register(e *echo.Echo) {
	for _, method := range webdavMethods {
		e.Add(method, WebDAVPrefix, h.serve)
		e.Add(method, WebDAVPrefix+"/*", h.serve)
	}
}

// serve authenticates with HTTP Basic, where the password is a personal
// access token with the webdav scope. The account password is never used.
func (h *WebDAVHandler) serve(c *echo.Context) error {
	request := c.Request()
	_, secret, ok := request.BasicAuth()
	if !ok {
		return challenge(c)
	}
	ctx := request.Context()
	token, user, err := h.auth.Tokens.Authenticate(ctx, secret)
	if err != nil || !service.HasScope(token, service.ScopeWebDAV) {
		return challenge(c)
	}
	if !slices.Contains(readOnlyMethods, request.Method) && !service.HasScope(token, service.ScopeWrite) {
		return respondError(c, http.StatusForbidden, "this token is read-only")
	}
	membership, err := h.auth.Orgs.Membership(ctx, token.OrgID, string(user.ID))
	if err != nil {
		return challenge(c)
	}
	actor := service.Actor{UserID: string(user.ID), OrgID: token.OrgID, OrgRole: membership.Role, Email: user.Email}
	h.dav.ServeHTTP(c.Response(), request.WithContext(service.WithActor(ctx, actor)))
	return nil
}

func challenge(c *echo.Context) error {
	c.Response().Header().Set("WWW-Authenticate", `Basic realm="EDMS", charset="UTF-8"`)
	return respondError(c, http.StatusUnauthorized, "use your email and a personal access token with the webdav scope")
}

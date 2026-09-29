package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/surrealdb/surrealdb.go"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
	"github.com/xmtrxi/edms/internal/signing"
)

type DocumentsHandler struct {
	cfg           config.Config
	docs          *service.DocumentService
	auth          *service.AuthService
	search        *service.SearchService
	categories    *service.CategoryService
	folderService *service.FolderService
	access        *service.AccessService
	db            *surrealdb.DB
	signing       signing.Provider
	signingSvc    *service.SigningService
}

func NewDocumentsHandler(deps Deps) *DocumentsHandler {
	handler := &DocumentsHandler{
		cfg:           deps.Cfg,
		docs:          service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger),
		auth:          service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		search:        service.NewSearchService(deps.Client(), deps.Storage, deps.Search, deps.Logger),
		categories:    service.NewCategoryService(deps.Client()),
		folderService: service.NewFolderService(deps.Client()),
		access:        service.NewAccessService(deps.Client()),
		db:            deps.Client(),
	}
	if deps.Cfg.Signing.Enabled && deps.Cfg.Signing.Provider == "documenso" {
		handler.signing = signing.NewDocumensoClient(deps.Cfg.Signing.DocumensoBaseURL, deps.Cfg.Signing.DocumensoAPIKey, nil)
		handler.signingSvc = service.NewSigningService(deps.Client(), deps.Storage, handler.signing)
	}
	return handler
}

func (h *DocumentsHandler) RegisterRoutes(api *echo.Group) {
	g := api.Group("/documents", middleware.RequireAuth(h.auth))
	g.GET("", h.list)
	g.POST("", h.upload)
	g.POST("/sync", h.sync)
	g.GET("/folders", h.folders)
	g.GET("/:id", h.get)
	g.GET("/:id/ocr", h.ocr)
	g.GET("/:id/processing-job", h.processingJob)
	g.POST("/:id/redo-ocr", h.redoOCR)
	g.GET("/:id/download", h.download)
	g.PUT("/:id/category", h.setCategory)
	g.DELETE("/:id", h.delete)
	if h.cfg.Signing.Enabled {
		g.POST("/:id/sign", h.sign)
		api.POST("/signing/webhook", h.signingWebhook)
	}
}

func (h *DocumentsHandler) signingWebhook(c *echo.Context) error {
	if h.signingSvc == nil || h.cfg.Signing.DocumensoWebhookSecret == "" {
		return respondError(c, http.StatusServiceUnavailable, "signing webhook is not configured")
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "could not read webhook")
	}
	signature := c.Request().Header.Get("X-Documenso-Signature")
	if signature == "" {
		signature = c.Request().Header.Get("X-Signature")
	}
	if !validWebhookSignature(body, signature, h.cfg.Signing.DocumensoWebhookSecret) {
		return respondError(c, http.StatusUnauthorized, "invalid webhook signature")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return respondError(c, http.StatusBadRequest, "invalid webhook payload")
	}
	event := strings.ToLower(firstWebhookString(payload, "event", "type", "status"))
	if !strings.Contains(event, "complete") && !strings.Contains(event, "signed") {
		return respondJSON(c, http.StatusAccepted, dto.WithMessage(map[string]any{}, "event ignored"))
	}
	providerDocumentID := firstWebhookString(payload, "provider_document_id", "document_id", "documentId", "id")
	if providerDocumentID == "" {
		return respondError(c, http.StatusBadRequest, "provider document id is required")
	}
	request, err := h.signingSvc.Complete(c.Request().Context(), providerDocumentID, firstWebhookString(payload, "download_url", "downloadUrl"))
	if err != nil {
		return respondError(c, http.StatusBadGateway, err.Error())
	}
	return respondJSON(c, http.StatusOK, dto.OK(request))
}

func validWebhookSignature(body []byte, signature, secret string) bool {
	signature = strings.TrimSpace(strings.TrimPrefix(signature, "sha256="))
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func firstWebhookString(payload map[string]any, keys ...string) string {
	for _, containerKey := range []string{"data", "payload", "document"} {
		if nested, ok := payload[containerKey].(map[string]any); ok {
			if value := firstWebhookString(nested, keys...); value != "" {
				return value
			}
		}
	}
	for _, key := range keys {
		switch value := payload[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		}
	}
	return ""
}

func (h *DocumentsHandler) list(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	page, pageSize := parsePageParams(c)

	// Restrict the listing to a category (and its subcategories) when asked.
	var documentIDs []string
	if categoryID := strings.TrimSpace(c.QueryParam("category_id")); categoryID != "" {
		ids, err := h.categories.DocumentIDs(c.Request().Context(), string(user.ID), categoryID)
		if err != nil {
			if errors.Is(err, service.ErrCategoryNotFound) {
				return respondError(c, http.StatusNotFound, "category not found")
			}
			return respondError(c, http.StatusInternalServerError, "failed to list documents")
		}
		if len(ids) == 0 {
			return respondJSON(c, http.StatusOK, dto.List([]dto.DocumentResponse{}, 0, page, pageSize))
		}
		documentIDs = ids
	}

	result, err := h.search.Search(c.Request().Context(), service.SearchInput{
		Query:       c.QueryParam("q"),
		OwnerID:     string(user.ID),
		Status:      c.QueryParam("status"),
		FolderPath:  strings.TrimSpace(c.QueryParam("folder_path")),
		FolderID:    strings.TrimSpace(c.QueryParam("folder_id")),
		DocumentIDs: documentIDs,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list documents")
	}

	ids := make([]string, 0, len(result.Items))
	for i := range result.Items {
		ids = append(ids, string(result.Items[i].Document.ID))
	}
	categories, err := h.categories.CategoriesForDocuments(c.Request().Context(), ids)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list documents")
	}

	ownerIDs := make([]string, 0, len(result.Items))
	for i := range result.Items {
		ownerIDs = append(ownerIDs, result.Items[i].Document.OwnerID)
	}
	owners := h.auth.Orgs.UsersByID(c.Request().Context(), ownerIDs)
	items := make([]dto.DocumentResponse, 0, len(result.Items))
	for i := range result.Items {
		doc := result.Items[i].Document
		items = append(items, dto.NewDocumentResponse(&doc).WithOwner(owners[doc.OwnerID]).WithCategory(categoryFor(categories, string(doc.ID))))
	}

	return respondJSON(c, http.StatusOK, dto.List(items, result.Total, result.Page, result.PageSize))
}

func (h *DocumentsHandler) folders(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	folders, err := h.docs.Folders(c.Request().Context(), string(user.ID))
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to list folders")
	}
	for i := range folders {
		for _, categoryID := range folders[i].CategoryIDs {
			category, err := h.categories.Get(c.Request().Context(), string(user.ID), categoryID)
			if err == nil {
				folders[i].Categories = append(folders[i].Categories, dto.NewCategoryResponse(category, 0))
			}
		}
	}
	return respondJSON(c, http.StatusOK, dto.OK(folders))
}

func categoryFor(categories map[string]dto.CategoryResponse, documentID string) *dto.CategoryResponse {
	category, ok := categories[documentID]
	if !ok {
		return nil
	}
	return &category
}

func (h *DocumentsHandler) upload(c *echo.Context) error {
	user := middleware.CurrentUser(c)

	file, err := c.FormFile("file")
	if err != nil {
		return respondError(c, http.StatusBadRequest, "a file is required")
	}

	src, err := file.Open()
	if err != nil {
		return respondError(c, http.StatusBadRequest, "could not read uploaded file")
	}
	defer src.Close()

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		name = file.Filename
	}
	name = filepath.Base(name)
	if name == "." || name == "" {
		return respondError(c, http.StatusBadRequest, "a valid filename is required")
	}

	mime := file.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}

	metadata := map[string]any{}
	if raw := strings.TrimSpace(c.FormValue("metadata")); raw != "" {
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return respondError(c, http.StatusBadRequest, "metadata must be a JSON object")
		}
		var ok bool
		metadata, ok = value.(map[string]any)
		if !ok {
			return respondError(c, http.StatusBadRequest, "metadata must be a JSON object")
		}
	}

	categoryID := strings.TrimSpace(c.FormValue("category_id"))
	folderID := strings.TrimSpace(c.FormValue("folder_id"))
	if folderID != "" {
		if err := h.access.RequireFolder(c.Request().Context(), string(user.ID), "upload", folderID); err != nil {
			return respondError(c, http.StatusForbidden, "you do not have upload access to this folder")
		}
	}
	access, status, message := h.uploadAccess(c, folderID)
	if status != 0 {
		return respondError(c, status, message)
	}
	if categoryID != "" {
		if _, err := h.categories.Get(c.Request().Context(), string(user.ID), categoryID); err != nil {
			return respondError(c, http.StatusBadRequest, "select a valid category")
		}
	}
	folderCategoryIDs := parseStringList(c.FormValue("folder_category_ids"))
	for _, id := range folderCategoryIDs {
		if _, err := h.categories.Get(c.Request().Context(), string(user.ID), id); err != nil {
			return respondError(c, http.StatusBadRequest, "select valid folder categories")
		}
	}
	doc, duplicate, err := h.docs.Upload(c.Request().Context(), service.UploadInput{
		OrgID:             middleware.CurrentOrgID(c),
		Access:            access,
		OwnerID:           string(user.ID),
		FolderID:          folderID,
		Name:              name,
		MimeType:          mime,
		FolderPath:        c.FormValue("folder_path"),
		FolderColor:       c.FormValue("folder_color"),
		FolderCategoryIDs: folderCategoryIDs,
		File:              src,
		Metadata:          metadata,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidGrant) {
			return respondError(c, http.StatusBadRequest, err.Error())
		}
		return respondError(c, http.StatusInternalServerError, "failed to upload document")
	}

	var category *dto.CategoryResponse
	if categoryID != "" {
		if err := h.categories.SetDocumentCategory(c.Request().Context(), string(user.ID), string(doc.ID), categoryID); err != nil {
			return respondError(c, http.StatusBadRequest, "could not assign the selected category")
		}
		categories, err := h.categories.CategoriesForDocuments(c.Request().Context(), []string{string(doc.ID)})
		if err == nil {
			category = categoryFor(categories, string(doc.ID))
		}
	}

	response := dto.NewDocumentResponse(doc).WithOwner(user).WithCategory(category).WithPermissions(h.permissions(c, doc))
	if duplicate {
		return respondJSON(c, http.StatusOK, dto.WithMessage(response, "duplicate file already exists; upload skipped"))
	}
	return respondJSON(c, http.StatusCreated, dto.Created(response))
}

// uploadAccess reads the optional "access" form field. Narrowing access is
// always allowed; giving anyone else access to a file in a folder needs the
// share capability on that folder, and guests can only upload privately or
// following the folder. A non-zero status means the request must be refused.
func (h *DocumentsHandler) uploadAccess(c *echo.Context, folderID string) (service.UploadAccess, int, string) {
	var access service.UploadAccess
	raw := strings.TrimSpace(c.FormValue("access"))
	if raw == "" {
		return access, 0, ""
	}
	if err := json.Unmarshal([]byte(raw), &access); err != nil {
		return access, http.StatusBadRequest, "access must be a JSON object"
	}
	switch access.Mode {
	case "", service.AccessModeInherit, service.AccessModePrivate:
		access.Grants = nil
	case service.AccessModeCustom:
		actor, _ := service.ActorFrom(c.Request().Context())
		if actor.IsGuest() && len(access.Grants) > 0 {
			return access, http.StatusForbidden, "guests cannot share files"
		}
		if folderID != "" && len(access.Grants) > 0 {
			if err := h.access.RequireFolder(c.Request().Context(), actor.UserID, "share", folderID); err != nil {
				return access, http.StatusForbidden, "you can upload here, but not share files from this folder with others"
			}
		}
	default:
		return access, http.StatusBadRequest, "access mode must be inherit, private or custom"
	}
	if access.Sensitivity != "" && !database.IsSensitivity(access.Sensitivity) {
		return access, http.StatusBadRequest, "unknown sensitivity label"
	}
	return access, 0, ""
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var values []string
	if strings.HasPrefix(raw, "[") && json.Unmarshal([]byte(raw), &values) == nil {
		return values
	}
	return strings.Split(raw, ",")
}

// sync imports files found in storage. Storage is shared by every
// organization on a SaaS install, so this only runs on self-hosted installs
// and only for organization admins.
func (h *DocumentsHandler) sync(c *echo.Context) error {
	if h.auth.Orgs.Capabilities().IsSaaS() {
		return respondError(c, http.StatusForbidden, "storage sync is not available on this installation")
	}
	if !middleware.IsOrgAdmin(c) {
		return respondError(c, http.StatusForbidden, "only organization admins can sync storage")
	}
	user := middleware.CurrentUser(c)
	workers := 4
	if raw := c.QueryParam("workers"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			workers = value
		}
	}
	dryRun, err := strconv.ParseBool(c.QueryParam("dry_run"))
	if err != nil && c.QueryParam("dry_run") != "" {
		return respondError(c, http.StatusBadRequest, "dry_run must be true or false")
	}
	result, err := h.docs.Sync(c.Request().Context(), string(user.ID), workers, dryRun)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to sync storage")
	}
	if dryRun {
		return respondJSON(c, http.StatusOK, dto.WithMessage(result, "storage dry run completed; no documents were imported or queued for OCR"))
	}
	return respondJSON(c, http.StatusAccepted, dto.WithMessage(result, "storage sync completed; imported documents were queued for OCR and indexing"))
}

func (h *DocumentsHandler) get(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, extraction, err := h.docs.GetFullOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	categories, err := h.categories.CategoriesForDocuments(c.Request().Context(), []string{string(doc.ID)})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "request failed")
	}
	response := dto.NewDocumentResponse(doc).WithOwner(h.owner(c, doc)).WithOCR(extraction).WithCategory(categoryFor(categories, string(doc.ID))).WithPermissions(h.permissions(c, doc))
	return respondJSON(c, http.StatusOK, dto.OK(response))
}

// setCategory moves a document into a category, or clears the assignment when
// category_id is empty.
func (h *DocumentsHandler) setCategory(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, err := h.docs.GetFor(c.Request().Context(), string(user.ID), "edit", c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}

	var request struct {
		CategoryID string `json:"category_id"`
	}
	if err := bind(c, &request); err != nil {
		return err
	}

	categoryID := strings.TrimSpace(request.CategoryID)
	if err := h.categories.SetDocumentCategory(c.Request().Context(), string(user.ID), string(doc.ID), categoryID); err != nil {
		if errors.Is(err, service.ErrCategoryNotFound) {
			return respondError(c, http.StatusNotFound, "category not found")
		}
		return respondError(c, http.StatusInternalServerError, "could not update the document category")
	}

	categories, err := h.categories.CategoriesForDocuments(c.Request().Context(), []string{string(doc.ID)})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "request failed")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewDocumentResponse(doc).WithOwner(h.owner(c, doc)).WithCategory(categoryFor(categories, string(doc.ID)))))
}

func (h *DocumentsHandler) ocr(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	loadFull, _ := strconv.ParseBool(c.QueryParam("full"))
	var extraction *database.DocumentExtraction
	var err error
	if loadFull {
		_, extraction, err = h.docs.GetFullOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	} else {
		_, extraction, err = h.docs.GetOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	}
	if err != nil {
		return h.handleError(c, err)
	}
	if extraction == nil {
		return respondError(c, http.StatusNotFound, "ocr not available for this document")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewOCRResponse(extraction)))
}

func (h *DocumentsHandler) processingJob(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	job, err := h.docs.LatestProcessingJob(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if job == nil {
		return respondError(c, http.StatusNotFound, "processing job not available for this document")
	}
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewProcessingJobResponse(job)))
}

func (h *DocumentsHandler) redoOCR(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, err := h.docs.ReOCR(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return respondJSON(c, http.StatusAccepted, dto.OK(dto.NewDocumentResponse(doc).WithOwner(h.owner(c, doc))))
}

func (h *DocumentsHandler) download(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	doc, reader, err := h.docs.Download(c.Request().Context(), string(user.ID), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	defer reader.Close()

	c.Response().Header().Set(echo.HeaderContentType, doc.MimeType)
	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename=\""+strings.ReplaceAll(doc.Name, "\"", "")+"\"")
	return c.Stream(http.StatusOK, doc.MimeType, reader)
}

func (h *DocumentsHandler) delete(c *echo.Context) error {
	user := middleware.CurrentUser(c)
	if err := h.docs.Delete(c.Request().Context(), string(user.ID), c.Param("id")); err != nil {
		return h.handleError(c, err)
	}
	if err := h.categories.RemoveDocument(c.Request().Context(), c.Param("id")); err != nil {
		return respondError(c, http.StatusInternalServerError, "request failed")
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *DocumentsHandler) sign(c *echo.Context) error {
	if h.signing == nil {
		return respondError(c, http.StatusServiceUnavailable, "signing provider is not configured")
	}
	var input struct {
		Title      string              `json:"title"`
		Recipients []signing.Recipient `json:"recipients"`
	}
	if err := bind(c, &input); err != nil {
		return err
	}
	if len(input.Recipients) == 0 {
		return respondError(c, http.StatusBadRequest, "at least one recipient is required")
	}
	document, reader, err := h.docs.Download(c.Request().Context(), actor(c), c.Param("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	defer reader.Close()
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = document.Name
	}
	result, err := h.signing.Create(c.Request().Context(), signing.CreateRequest{Title: title, FileName: document.Name, File: reader, Recipients: input.Recipients, ExternalID: string(document.ID)})
	if err != nil {
		return respondError(c, http.StatusBadGateway, err.Error())
	}
	now := time.Now().UTC()
	request, err := surrealdb.Create[database.SigningRequest](c.Request().Context(), h.db, "signing_requests", &database.SigningRequest{DocumentID: string(document.ID), DocumentVersionID: document.CurrentVersionID, RequestedBy: actor(c), Provider: h.cfg.Signing.Provider, ProviderDocumentID: result.DocumentID, SigningURL: result.SigningURL, Status: database.SigningStatusSent, CreatedAt: now, UpdatedAt: now})
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "failed to save signing request")
	}
	return respondJSON(c, http.StatusCreated, dto.Created(request))
}

func (h *DocumentsHandler) owner(c *echo.Context, doc *database.Document) *database.User {
	return h.auth.Orgs.UsersByID(c.Request().Context(), []string{doc.OwnerID})[doc.OwnerID]
}

func (h *DocumentsHandler) permissions(c *echo.Context, doc *database.Document) []string {
	permissions, err := h.access.Capabilities(c.Request().Context(), actor(c), doc)
	if err != nil {
		return nil
	}
	return permissions
}

func (h *DocumentsHandler) handleError(c *echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		return respondError(c, http.StatusNotFound, "document not found")
	case errors.Is(err, service.ErrForbidden):
		return respondError(c, http.StatusForbidden, "forbidden")
	default:
		return respondError(c, http.StatusInternalServerError, "request failed")
	}
}

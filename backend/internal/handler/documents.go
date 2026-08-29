package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/config"
	"github.com/xmtrxi/edms/internal/database"
	"github.com/xmtrxi/edms/internal/dto"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

type DocumentsHandler struct {
	cfg           config.Config
	docs          *service.DocumentService
	auth          *service.AuthService
	search        *service.SearchService
	categories    *service.CategoryService
	folderService *service.FolderService
	access        *service.AccessService
}

func NewDocumentsHandler(deps Deps) *DocumentsHandler {
	return &DocumentsHandler{
		cfg:           deps.Cfg,
		docs:          service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger),
		auth:          service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		search:        service.NewSearchService(deps.Client(), deps.Storage, deps.Search, deps.Logger),
		categories:    service.NewCategoryService(deps.Client()),
		folderService: service.NewFolderService(deps.Client()),
		access:        service.NewAccessService(deps.Client()),
	}
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
	}
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

	items := make([]dto.DocumentResponse, 0, len(result.Items))
	for i := range result.Items {
		doc := result.Items[i].Document
		items = append(items, dto.NewDocumentResponse(&doc).WithOwner(user).WithCategory(categoryFor(categories, string(doc.ID))))
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

	response := dto.NewDocumentResponse(doc).WithOwner(user).WithCategory(category)
	if duplicate {
		return respondJSON(c, http.StatusOK, dto.WithMessage(response, "duplicate file already exists; upload skipped"))
	}
	return respondJSON(c, http.StatusCreated, dto.Created(response))
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

func (h *DocumentsHandler) sync(c *echo.Context) error {
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
	response := dto.NewDocumentResponse(doc).WithOwner(user).WithOCR(extraction).WithCategory(categoryFor(categories, string(doc.ID)))
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
	return respondJSON(c, http.StatusOK, dto.OK(dto.NewDocumentResponse(doc).WithOwner(user).WithCategory(categoryFor(categories, string(doc.ID)))))
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
	return respondJSON(c, http.StatusAccepted, dto.OK(dto.NewDocumentResponse(doc).WithOwner(user)))
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
	return respondJSON(c, http.StatusNotImplemented, dto.WithMessage(map[string]string{}, "document signing is enabled but not implemented yet"))
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

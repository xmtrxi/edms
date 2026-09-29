package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v5"

	applicationai "github.com/xmtrxi/edms/internal/ai"
	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/service"
)

// mcpProtocolVersions are the Model Context Protocol revisions this server speaks.
var mcpProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// MCPHandler lets AI agents use the drive through the Model Context Protocol.
// It answers JSON-RPC over plain HTTP POST (Streamable HTTP without
// server-initiated streams). Agents authenticate with a personal access token
// that has the mcp scope and act with that account's access.
type MCPHandler struct {
	auth    *service.AuthService
	docs    *service.DocumentService
	folders *service.FolderService
	search  *service.SearchService
	ai      *service.AIService
}

func NewMCPHandler(deps Deps) *MCPHandler {
	db := deps.Client()
	search := service.NewSearchService(db, deps.Storage, deps.Search, deps.Logger)
	return &MCPHandler{
		auth:    service.NewAuthService(deps.Cfg, db, deps.Logger),
		docs:    service.NewDocumentService(deps.Cfg, db, deps.Storage, deps.Queue, deps.Search, deps.Logger),
		folders: service.NewFolderService(db),
		search:  search,
		ai:      service.NewAIService(db, deps.Cfg.AI).WithSearch(search),
	}
}

func (h *MCPHandler) RegisterRoutes(api *echo.Group) {
	api.POST("/mcp", h.serve, middleware.RequireAuth(h.auth))
	// This server never opens a server-to-client stream.
	api.GET("/mcp", func(c *echo.Context) error { return c.NoContent(http.StatusMethodNotAllowed) })
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (h *MCPHandler) serve(c *echo.Context) error {
	token := middleware.CurrentAPIToken(c)
	if token != nil && !service.HasScope(token, service.ScopeMCP) {
		return respondError(c, http.StatusForbidden, "this token does not have the mcp scope")
	}
	var req rpcRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
	}
	// Notifications carry no id and get no response body.
	if len(req.ID) == 0 {
		return c.NoContent(http.StatusAccepted)
	}
	result, rerr := h.dispatch(c, req)
	response := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr}
	return c.JSON(http.StatusOK, response)
}

func (h *MCPHandler) dispatch(c *echo.Context, req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		version := mcpProtocolVersions[0]
		if slices.Contains(mcpProtocolVersions, params.ProtocolVersion) {
			version = params.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "edms", "title": "Lumen DMS", "version": "1.0.0"},
			"instructions": "Search, read and answer questions from the documents this account can open. " +
				"Cite document names when you use their content. Sharing and deleting are not available to agents.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": mcpTools()}, nil
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		text, err := h.call(c, params.Name, params.Arguments)
		if err != nil {
			var unknown unknownToolError
			if errors.As(err, &unknown) {
				return nil, &rpcError{Code: -32602, Message: err.Error()}
			}
			// Tool failures are results the model can read and react to.
			return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true}, nil
		}
		return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}, nil
	}
	return nil, &rpcError{Code: -32601, Message: "method not found"}
}

type unknownToolError struct{ name string }

func (e unknownToolError) Error() string { return "unknown tool " + e.name }

func schema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func mcpTools() []map[string]any {
	readOnly := map[string]any{"readOnlyHint": true, "openWorldHint": false}
	return []map[string]any{
		{
			"name": "search_documents", "title": "Search documents",
			"description": "Full-text search over documents this account can open. Returns names, ids and matching snippets.",
			"inputSchema": schema(map[string]any{
				"query": str("Words or phrases to search for"),
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "Maximum results (default 10)"},
			}, "query"),
			"annotations": readOnly,
		},
		{
			"name": "list_folder", "title": "List a folder",
			"description": "List subfolders and files in a folder. Omit folder_id for the top level and My files.",
			"inputSchema": schema(map[string]any{"folder_id": str("Folder id from a previous listing")}),
			"annotations": readOnly,
		},
		{
			"name": "read_document", "title": "Read a document",
			"description": "Return a document's details and its extracted text.",
			"inputSchema": schema(map[string]any{
				"document_id": str("Document id"),
				"max_chars":   map[string]any{"type": "integer", "minimum": 500, "maximum": 200000, "description": "Maximum characters of text (default 20000)"},
			}, "document_id"),
			"annotations": readOnly,
		},
		{
			"name": "answer_question", "title": "Answer from documents",
			"description": "Answer a question using only documents this account can open, with numbered citations.",
			"inputSchema": schema(map[string]any{
				"question":     str("The question"),
				"folder_id":    str("Limit to this folder and its subfolders"),
				"document_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Limit to these documents"},
			}, "question"),
			"annotations": readOnly,
		},
		{
			"name": "create_text_document", "title": "Create a text document",
			"description": "Save text as a new document, for example a report or notes. Needs a token with write scope.",
			"inputSchema": schema(map[string]any{
				"name":      str("File name, e.g. summary.md"),
				"content":   str("The document text"),
				"folder_id": str("Folder to save into; omit for My files"),
			}, "name", "content"),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false},
		},
		{
			"name": "move_document", "title": "Move a document",
			"description": "Move a document into a folder the account can add to. Needs a token with write scope.",
			"inputSchema": schema(map[string]any{
				"document_id": str("Document id"),
				"folder_id":   str("Destination folder id, or \"root\" for My files"),
			}, "document_id", "folder_id"),
			"annotations": map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true},
		},
	}
}

func (h *MCPHandler) call(c *echo.Context, name string, raw json.RawMessage) (string, error) {
	ctx := c.Request().Context()
	user := actor(c)
	writes := map[string]bool{"create_text_document": true, "move_document": true}
	if writes[name] && !service.HasScope(middleware.CurrentAPIToken(c), service.ScopeWrite) {
		return "", fmt.Errorf("this token is read-only; ask for a token with write scope")
	}
	decode := func(target any) error {
		if len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, target); err != nil {
			return fmt.Errorf("invalid arguments: %w", err)
		}
		return nil
	}
	switch name {
	case "search_documents":
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := decode(&args); err != nil {
			return "", err
		}
		if args.Limit <= 0 || args.Limit > 50 {
			args.Limit = 10
		}
		result, err := h.search.Search(ctx, service.SearchInput{Query: args.Query, OwnerID: user, Page: 1, PageSize: args.Limit})
		if err != nil {
			return "", err
		}
		items := []map[string]any{}
		for _, item := range result.Items {
			items = append(items, map[string]any{
				"id": string(item.Document.ID), "name": item.Document.Name, "folder_id": item.Document.FolderID,
				"updated_at": item.Document.UpdatedAt, "snippet": item.Snippet,
			})
		}
		return toJSON(map[string]any{"total": result.Total, "results": items})
	case "list_folder":
		var args struct {
			FolderID string `json:"folder_id"`
		}
		if err := decode(&args); err != nil {
			return "", err
		}
		folders, err := h.folders.List(ctx, user)
		if err != nil {
			return "", err
		}
		subfolders := []map[string]any{}
		for _, folder := range folders {
			if folder.ParentID == args.FolderID {
				subfolders = append(subfolders, map[string]any{"id": folder.ID, "name": folder.Name, "files": folder.DocumentCount})
			}
		}
		folderID := args.FolderID
		if folderID == "" {
			folderID = service.RootFolderID
		}
		result, err := h.search.Search(ctx, service.SearchInput{OwnerID: user, FolderID: folderID, Page: 1, PageSize: 200})
		if err != nil {
			return "", err
		}
		files := []map[string]any{}
		for _, item := range result.Items {
			files = append(files, map[string]any{"id": string(item.Document.ID), "name": item.Document.Name, "size": item.Document.Size, "updated_at": item.Document.UpdatedAt})
		}
		return toJSON(map[string]any{"folders": subfolders, "files": files})
	case "read_document":
		var args struct {
			DocumentID string `json:"document_id"`
			MaxChars   int    `json:"max_chars"`
		}
		if err := decode(&args); err != nil {
			return "", err
		}
		if args.MaxChars <= 0 {
			args.MaxChars = 20_000
		}
		doc, extraction, err := h.docs.GetFullOCR(ctx, user, args.DocumentID)
		if err != nil {
			return "", fmt.Errorf("document not found or not shared with this account")
		}
		text := ""
		if extraction != nil {
			text = extraction.Text
		}
		truncated := len(text) > args.MaxChars
		if truncated {
			text = text[:args.MaxChars]
		}
		return toJSON(map[string]any{
			"id": string(doc.ID), "name": doc.Name, "mime_type": doc.MimeType, "size": doc.Size,
			"folder_id": doc.FolderID, "sensitivity": doc.Sensitivity, "updated_at": doc.UpdatedAt,
			"summary": doc.Metadata["ai_summary"], "text": text, "truncated": truncated,
		})
	case "answer_question":
		var args service.AnswerInput
		if err := decode(&args); err != nil {
			return "", err
		}
		result, err := h.ai.Answer(ctx, user, args)
		if errors.Is(err, applicationai.ErrDisabled) {
			return "", fmt.Errorf("AI answering is not configured; use search_documents and read_document instead")
		}
		if err != nil {
			return "", err
		}
		return toJSON(result)
	case "create_text_document":
		var args struct {
			Name     string `json:"name"`
			Content  string `json:"content"`
			FolderID string `json:"folder_id"`
		}
		if err := decode(&args); err != nil {
			return "", err
		}
		if args.FolderID != "" {
			if err := service.NewAccessService(h.folders.DB).RequireFolder(ctx, user, "upload", args.FolderID); err != nil {
				return "", fmt.Errorf("this account cannot add files to that folder")
			}
		}
		name := strings.TrimSpace(args.Name)
		mime := "text/plain"
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			mime = "text/markdown"
		}
		doc, _, err := h.docs.Upload(ctx, service.UploadInput{
			OwnerID: user, FolderID: args.FolderID, Name: name, MimeType: mime,
			File: strings.NewReader(args.Content), Metadata: map[string]any{"source": "mcp_agent"},
		})
		if err != nil {
			return "", err
		}
		return toJSON(map[string]any{"id": string(doc.ID), "name": doc.Name})
	case "move_document":
		var args struct {
			DocumentID string `json:"document_id"`
			FolderID   string `json:"folder_id"`
		}
		if err := decode(&args); err != nil {
			return "", err
		}
		doc, err := h.docs.Update(ctx, user, args.DocumentID, service.DocumentUpdate{FolderID: &args.FolderID})
		if err != nil {
			return "", fmt.Errorf("could not move the document: %w", err)
		}
		return toJSON(map[string]any{"id": string(doc.ID), "folder_id": doc.FolderID})
	}
	return "", unknownToolError{name: name}
}

func toJSON(value any) (string, error) {
	payload, err := json.MarshalIndent(value, "", "  ")
	return string(payload), err
}

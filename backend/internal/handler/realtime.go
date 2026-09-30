package handler

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"

	"github.com/xmtrxi/edms/internal/middleware"
	"github.com/xmtrxi/edms/internal/realtime"
	"github.com/xmtrxi/edms/internal/service"
)

// RealtimeHandler upgrades signed-in browsers to a WebSocket that carries
// live events for the documents and folders they have open.
type RealtimeHandler struct {
	auth     *service.AuthService
	docs     *service.DocumentService
	access   *service.AccessService
	hub      *realtime.Hub
	upgrader websocket.Upgrader
}

func NewRealtimeHandler(deps Deps) *RealtimeHandler {
	allowed := map[string]bool{"localhost:3000": true, "127.0.0.1:3000": true, "localhost:3001": true}
	if app, err := url.Parse(deps.Cfg.SMTP.AppURL); err == nil && app.Host != "" {
		allowed[app.Host] = true
	}
	return &RealtimeHandler{
		auth:   service.NewAuthService(deps.Cfg, deps.Client(), deps.Logger),
		docs:   service.NewDocumentService(deps.Cfg, deps.Client(), deps.Storage, deps.Queue, deps.Search, deps.Logger),
		access: service.NewAccessService(deps.Client()),
		hub:    deps.Realtime,
		upgrader: websocket.Upgrader{
			Subprotocols: []string{middleware.WebSocketProtocol},
			CheckOrigin: func(r *http.Request) bool {
				origin, err := url.Parse(r.Header.Get("Origin"))
				return err == nil && (origin.Host == r.Host || allowed[origin.Host])
			},
		},
	}
}

func (h *RealtimeHandler) RegisterRoutes(api *echo.Group) {
	api.GET("/realtime", h.connect, middleware.RequireAuth(h.auth))
}

type realtimeCommand struct {
	Op    string `json:"op"`
	Topic string `json:"topic"`
}

func (h *RealtimeHandler) connect(c *echo.Context) error {
	if h.hub == nil {
		return respondError(c, http.StatusServiceUnavailable, "live updates are not available")
	}
	conn, err := h.upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return nil
	}
	defer conn.Close()

	user := middleware.CurrentUser(c)
	userID := string(user.ID)
	// The request context carries the actor used for access checks.
	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()
	client := h.hub.Connect(realtime.Person{ID: userID, Name: firstNonEmptyString(user.Name, user.Email), Email: user.Email})
	defer h.hub.Disconnect(context.Background(), client)
	h.hub.Subscribe(ctx, client, "user:"+userID)

	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(75 * time.Second)) })

	go func() {
		defer cancel()
		for {
			var command realtimeCommand
			if err := conn.ReadJSON(&command); err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
			switch command.Op {
			case "subscribe":
				if h.allowed(ctx, userID, command.Topic) {
					h.hub.Subscribe(ctx, client, command.Topic)
				}
			case "unsubscribe":
				h.hub.Unsubscribe(ctx, client, command.Topic)
			}
		}
	}()

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-client.Events():
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(event); err != nil {
				return nil
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				return nil
			}
		}
	}
}

// allowed checks the user may follow a topic: their own feed, or a document
// or folder they can view.
func (h *RealtimeHandler) allowed(ctx context.Context, userID, topic string) bool {
	kind, id, ok := strings.Cut(topic, ":")
	if !ok || id == "" {
		return false
	}
	switch kind {
	case "document":
		_, err := h.docs.GetFor(ctx, userID, "view", id)
		return err == nil
	case "folder":
		return h.access.RequireFolder(ctx, userID, "view", id) == nil
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

package service

import (
	"context"
	"sync"

	"github.com/xmtrxi/edms/internal/database"
)

// Actor is the signed-in user acting inside one organization. The auth
// middleware stores it on the request context, so services can scope queries
// to the organization without every call passing it explicitly.
type Actor struct {
	UserID  string
	OrgID   string
	OrgRole string
	Email   string
}

// IsAdmin reports whether the actor is an owner or admin of the organization.
func (a Actor) IsAdmin() bool {
	return a.OrgRole == database.MembershipRoleOwner || a.OrgRole == database.MembershipRoleAdmin
}

// IsGuest reports whether the actor only sees what is shared with them directly.
func (a Actor) IsGuest() bool { return a.OrgRole == database.MembershipRoleGuest }

type actorKey struct{}

// actorState memoizes the actor's principals for the life of one request, since
// a listing can check access for many documents.
type actorState struct {
	actor      Actor
	once       sync.Once
	principals *Principals
	err        error
}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, &actorState{actor: actor})
}

func ActorFrom(ctx context.Context) (Actor, bool) {
	state, ok := ctx.Value(actorKey{}).(*actorState)
	if !ok || state == nil {
		return Actor{}, false
	}
	return state.actor, true
}

// orgOf is the organization the request acts in, or "" outside a request.
func orgOf(ctx context.Context) string {
	actor, _ := ActorFrom(ctx)
	return actor.OrgID
}

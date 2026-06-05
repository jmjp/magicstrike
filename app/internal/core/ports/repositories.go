package ports

import (
	"context"

	"magicstrike/internal/core/entities"
)

// UserRepository defines the persistence interface for the User entity.
type UserRepository interface {
	Save(ctx context.Context, user *entities.User) error
	FindByID(ctx context.Context, id string) (*entities.User, error)
	FindByEmail(ctx context.Context, email string) (*entities.User, error)
	FindByUsername(ctx context.Context, username string) (*entities.User, error)
	Update(ctx context.Context, user *entities.User) error
}

// MatchRepository defines the persistence interface for the Match entity.
type MatchRepository interface {
	Save(ctx context.Context, match *entities.Match) error
	FindByID(ctx context.Context, id string) (*entities.Match, error)
	FindByDemoMD5(ctx context.Context, md5Hash string) (*entities.Match, error)
	Update(ctx context.Context, match *entities.Match) error
	List(ctx context.Context, limit, offset int) ([]*entities.Match, error)
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error)
}

// SessionRepository defines the persistence interface for user Sessions.
type SessionRepository interface {
	Save(ctx context.Context, session *entities.Session) error
	FindByID(ctx context.Context, id string) (*entities.Session, error)
	Delete(ctx context.Context, id string) error
}

// EventRepository defines the persistence interface for game Events.
type EventRepository interface {
	Save(ctx context.Context, event *entities.Event) error
	SaveBatch(ctx context.Context, events []*entities.Event) error
	FindByMatchID(ctx context.Context, matchID string) ([]*entities.Event, error)
	DeleteByMatchID(ctx context.Context, matchID string) error
}

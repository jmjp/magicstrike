package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
	"magicstrike/internal/core/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepo struct {
	ports.UserRepository
	findByEmailFn func(ctx context.Context, email string) (*entities.User, error)
	saveFn        func(ctx context.Context, user *entities.User) error
	findByIDFn    func(ctx context.Context, id string) (*entities.User, error)
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*entities.User, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *mockUserRepo) Save(ctx context.Context, user *entities.User) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*entities.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

type mockSessionRepo struct {
	ports.SessionRepository
	saveFn     func(ctx context.Context, session *entities.Session) error
	findByIDFn func(ctx context.Context, id string) (*entities.Session, error)
	deleteFn   func(ctx context.Context, id string) error
}

func (m *mockSessionRepo) Save(ctx context.Context, session *entities.Session) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, session)
	}
	return nil
}

func (m *mockSessionRepo) FindByID(ctx context.Context, id string) (*entities.Session, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockSessionRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

type mockMagicLinkStore struct {
	ports.MagicLinkStore
	storeFn    func(ctx context.Context, token string, data ports.MagicLinkData) error
	retrieveFn func(ctx context.Context, token string) (*ports.MagicLinkData, error)
	deleteFn   func(ctx context.Context, token string) error
}

func (m *mockMagicLinkStore) Store(ctx context.Context, token string, data ports.MagicLinkData) error {
	if m.storeFn != nil {
		return m.storeFn(ctx, token, data)
	}
	return nil
}

func (m *mockMagicLinkStore) Retrieve(ctx context.Context, token string) (*ports.MagicLinkData, error) {
	if m.retrieveFn != nil {
		return m.retrieveFn(ctx, token)
	}
	return nil, nil
}

func (m *mockMagicLinkStore) Delete(ctx context.Context, token string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, token)
	}
	return nil
}

type mockEmailSender struct {
	ports.EmailSender
	sendFn func(ctx context.Context, email ports.EmailAddress, token string) error
}

func (m *mockEmailSender) SendMagicLink(ctx context.Context, email ports.EmailAddress, token string) error {
	if m.sendFn != nil {
		return m.sendFn(ctx, email, token)
	}
	return nil
}

func TestAuthUseCase_EdgeCases_RequestMagicLink(t *testing.T) {
	ctx := context.Background()
	tokenGen := services.NewTokenGenerator()
	jwtSvc := services.NewJWTService("my-secret-key-1234567890")

	t.Run("rate limiter blocks", func(t *testing.T) {
		limiter := services.NewRateLimiter()
		// Trigger rate limit: call 6 times
		for i := 0; i < 5; i++ {
			limiter.Allow("user@example.com")
		}
		// 6th call should block
		if limiter.Allow("user@example.com") {
			t.Fatal("expected 6th allow call to return false")
		}

		uc := NewAuthUseCase(nil, nil, nil, nil, tokenGen, limiter, jwtSvc)
		err := uc.RequestMagicLink(ctx, "user@example.com")
		require.NoError(t, err) // always returns nil
	})

	t.Run("magic store store fails", func(t *testing.T) {
		limiter := services.NewRateLimiter()
		store := &mockMagicLinkStore{storeFn: func(ctx context.Context, token string, data ports.MagicLinkData) error {
			return errors.New("store failed")
		}}
		uc := NewAuthUseCase(nil, nil, store, nil, tokenGen, limiter, jwtSvc)
		err := uc.RequestMagicLink(ctx, "user@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to store magic link")
	})

	t.Run("email sender fails", func(t *testing.T) {
		limiter := services.NewRateLimiter()
		store := &mockMagicLinkStore{}
		sender := &mockEmailSender{sendFn: func(ctx context.Context, email ports.EmailAddress, token string) error {
			return errors.New("send failed")
		}}
		uc := NewAuthUseCase(nil, nil, store, sender, tokenGen, limiter, jwtSvc)
		err := uc.RequestMagicLink(ctx, "user@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to send magic link")
	})
}

func TestAuthUseCase_EdgeCases_ValidateToken(t *testing.T) {
	ctx := context.Background()
	tokenGen := services.NewTokenGenerator()
	limiter := services.NewRateLimiter()
	jwtSvc := services.NewJWTService("my-secret-key-1234567890")

	t.Run("retrieve fails", func(t *testing.T) {
		store := &mockMagicLinkStore{retrieveFn: func(ctx context.Context, token string) (*ports.MagicLinkData, error) {
			return nil, errors.New("retrieve failed")
		}}
		uc := NewAuthUseCase(nil, nil, store, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.ValidateToken(ctx, "tok")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve token")
	})

	t.Run("delete fails during validate", func(t *testing.T) {
		store := &mockMagicLinkStore{
			retrieveFn: func(ctx context.Context, token string) (*ports.MagicLinkData, error) {
				return &ports.MagicLinkData{Email: "user@example.com", ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
			},
			deleteFn: func(ctx context.Context, token string) error {
				return errors.New("delete failed")
			},
		}
		uc := NewAuthUseCase(nil, nil, store, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.ValidateToken(ctx, "tok")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete token")
	})

	t.Run("user find fails", func(t *testing.T) {
		store := &mockMagicLinkStore{
			retrieveFn: func(ctx context.Context, token string) (*ports.MagicLinkData, error) {
				return &ports.MagicLinkData{Email: "user@example.com", ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
			},
		}
		userRepo := &mockUserRepo{findByEmailFn: func(ctx context.Context, email string) (*entities.User, error) {
			return nil, errors.New("find user failed")
		}}
		uc := NewAuthUseCase(userRepo, nil, store, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.ValidateToken(ctx, "tok")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find user")
	})

	t.Run("new user save fails", func(t *testing.T) {
		store := &mockMagicLinkStore{
			retrieveFn: func(ctx context.Context, token string) (*ports.MagicLinkData, error) {
				return &ports.MagicLinkData{Email: "ab@example.com", ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
			},
		}
		userRepo := &mockUserRepo{
			findByEmailFn: func(ctx context.Context, email string) (*entities.User, error) {
				return nil, nil // user does not exist
			},
			saveFn: func(ctx context.Context, user *entities.User) error {
				return errors.New("save failed")
			},
		}
		uc := NewAuthUseCase(userRepo, nil, store, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.ValidateToken(ctx, "tok")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save user")
	})

	t.Run("session save fails", func(t *testing.T) {
		store := &mockMagicLinkStore{
			retrieveFn: func(ctx context.Context, token string) (*ports.MagicLinkData, error) {
				return &ports.MagicLinkData{Email: "user@example.com", ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
			},
		}
		userRepo := &mockUserRepo{
			findByEmailFn: func(ctx context.Context, email string) (*entities.User, error) {
				return &entities.User{ID: "user-1", Email: "user@example.com"}, nil
			},
		}
		sessionRepo := &mockSessionRepo{
			saveFn: func(ctx context.Context, session *entities.Session) error {
				return errors.New("session save failed")
			},
		}
		uc := NewAuthUseCase(userRepo, sessionRepo, store, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.ValidateToken(ctx, "tok")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save session")
	})
}

func TestAuthUseCase_EdgeCases_RefreshSession(t *testing.T) {
	ctx := context.Background()
	tokenGen := services.NewTokenGenerator()
	limiter := services.NewRateLimiter()
	jwtSvc := services.NewJWTService("my-secret-key-1234567890")

	t.Run("session find fails", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Session, error) {
				return nil, errors.New("find session failed")
			},
		}
		uc := NewAuthUseCase(nil, sessionRepo, nil, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.RefreshSession(ctx, "sess")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find session")
	})

	t.Run("session expired", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Session, error) {
				return &entities.Session{ID: "sess", ExpiresAt: time.Now().Add(-1 * time.Hour)}, nil
			},
		}
		uc := NewAuthUseCase(nil, sessionRepo, nil, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.RefreshSession(ctx, "sess")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid or expired session")
	})

	t.Run("session refresh save fails", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Session, error) {
				return &entities.Session{ID: "sess", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
			},
			saveFn: func(ctx context.Context, session *entities.Session) error {
				return errors.New("save failed")
			},
		}
		uc := NewAuthUseCase(nil, sessionRepo, nil, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.RefreshSession(ctx, "sess")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to refresh session")
	})

	t.Run("user find fails during refresh", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Session, error) {
				return &entities.Session{ID: "sess", UserID: "user-1", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
			},
		}
		userRepo := &mockUserRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.User, error) {
				return nil, errors.New("find user failed")
			},
		}
		uc := NewAuthUseCase(userRepo, sessionRepo, nil, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.RefreshSession(ctx, "sess")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find user")
	})

	t.Run("user nil during refresh", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.Session, error) {
				return &entities.Session{ID: "sess", UserID: "user-1", ExpiresAt: time.Now().Add(1 * time.Hour)}, nil
			},
		}
		userRepo := &mockUserRepo{
			findByIDFn: func(ctx context.Context, id string) (*entities.User, error) {
				return nil, nil // user deleted?
			},
		}
		uc := NewAuthUseCase(userRepo, sessionRepo, nil, nil, tokenGen, limiter, jwtSvc)
		_, err := uc.RefreshSession(ctx, "sess")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found for session")
	})
}

func TestAuthUseCase_EdgeCases_Logout(t *testing.T) {
	ctx := context.Background()
	tokenGen := services.NewTokenGenerator()
	limiter := services.NewRateLimiter()
	jwtSvc := services.NewJWTService("my-secret-key-1234567890")

	t.Run("delete session fails", func(t *testing.T) {
		sessionRepo := &mockSessionRepo{
			deleteFn: func(ctx context.Context, id string) error {
				return errors.New("delete session failed")
			},
		}
		uc := NewAuthUseCase(nil, sessionRepo, nil, nil, tokenGen, limiter, jwtSvc)
		err := uc.Logout(ctx, "sess")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete session")
	})
}

func TestDeriveUsername_Direct(t *testing.T) {
	assert.Equal(t, "player", deriveUsername("player@example.com"))
	assert.Equal(t, "abx", deriveUsername("ab@example.com"))
	assert.Equal(t, "axx", deriveUsername("a@example.com"))
	assert.Equal(t, "no-at-sign", deriveUsername("no-at-sign"))
	assert.Equal(t, "axx", deriveUsername("a"))
}

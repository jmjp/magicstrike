package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUserRepositoryWithErrors struct {
	ports.UserRepository
	users   map[string]*entities.User
	findErr error
}

func (m *mockUserRepositoryWithErrors) FindByID(ctx context.Context, id string) (*entities.User, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.users[id], nil
}

type mockMatchRepositoryWithErrors struct {
	ports.MatchRepository
	matches          map[string]*entities.Match
	md5Index         map[string]string
	findByIDErr      error
	findByDemoMD5Err error
	saveErr          error
	updateErr        error
}

func (m *mockMatchRepositoryWithErrors) Save(ctx context.Context, match *entities.Match) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.matches[match.ID] = match
	if match.DemoMD5 != nil {
		m.md5Index[*match.DemoMD5] = match.ID
	}
	return nil
}

func (m *mockMatchRepositoryWithErrors) FindByID(ctx context.Context, id string) (*entities.Match, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	return m.matches[id], nil
}

func (m *mockMatchRepositoryWithErrors) FindByDemoMD5(ctx context.Context, md5Hash string) (*entities.Match, error) {
	if m.findByDemoMD5Err != nil {
		return nil, m.findByDemoMD5Err
	}
	id, ok := m.md5Index[md5Hash]
	if !ok {
		return nil, nil
	}
	return m.matches[id], nil
}

func (m *mockMatchRepositoryWithErrors) Update(ctx context.Context, match *entities.Match) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.matches[match.ID] = match
	return nil
}

func TestRequestUpload_EdgeCases(t *testing.T) {
	ctx := context.Background()
	validMD5 := "d41d8cd98f00b204e9800998ecf8427e"

	t.Run("validation failure", func(t *testing.T) {
		uc := NewUploadUseCase(nil, nil, nil, nil, "bucket", 15*time.Minute)
		// Invalid request: empty UserID
		req := &ports.UploadRequest{
			UserID:   "",
			Filename: "demo.dem",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ports.ErrInvalidUploadRequest.Error())
	})

	t.Run("user repo find error", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users:   make(map[string]*entities.User),
			findErr: errors.New("db error"),
		}
		uc := NewUploadUseCase(nil, userRepo, nil, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify user")
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: make(map[string]*entities.User),
		} // empty, no user-1
		uc := NewUploadUseCase(nil, userRepo, nil, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("duplicate check repo error", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: map[string]*entities.User{
				"user-1": {ID: "user-1"},
			},
		}
		matchRepo := &mockMatchRepositoryWithErrors{
			findByDemoMD5Err: errors.New("db find error"),
		}

		uc := NewUploadUseCase(matchRepo, userRepo, nil, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
			MD5Hash:  &validMD5,
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check for duplicate")
	})

	t.Run("existing match find repo error", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: map[string]*entities.User{
				"user-1": {ID: "user-1"},
			},
		}
		matchRepo := &mockMatchRepositoryWithErrors{
			findByIDErr: errors.New("db error"),
		}

		uc := NewUploadUseCase(matchRepo, userRepo, nil, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
			MatchID:  "match-1",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check for existing match")
	})

	t.Run("existing match ownership error", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: map[string]*entities.User{
				"user-1": {ID: "user-1"},
			},
		}
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "other-user"},
			},
		}

		uc := NewUploadUseCase(matchRepo, userRepo, nil, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
			MatchID:  "match-1",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "belongs to another user")
	})

	t.Run("existing match presign url generation error", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: map[string]*entities.User{
				"user-1": {ID: "user-1"},
			},
		}
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "user-1"},
			},
		}
		storage := &mockStorageService{presignErr: errors.New("storage issue")}

		uc := NewUploadUseCase(matchRepo, userRepo, storage, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
			MatchID:  "match-1",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate presigned URL")
	})

	t.Run("match save fails", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: map[string]*entities.User{
				"user-1": {ID: "user-1"},
			},
		}
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: make(map[string]*entities.Match),
			saveErr: errors.New("save failed"),
		}

		uc := NewUploadUseCase(matchRepo, userRepo, nil, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save match")
	})

	t.Run("new match presign url fails", func(t *testing.T) {
		userRepo := &mockUserRepositoryWithErrors{
			users: map[string]*entities.User{
				"user-1": {ID: "user-1"},
			},
		}
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: make(map[string]*entities.Match),
		}
		storage := &mockStorageService{presignErr: errors.New("storage error")}

		uc := NewUploadUseCase(matchRepo, userRepo, storage, nil, "bucket", 15*time.Minute)
		req := &ports.UploadRequest{
			UserID:   "user-1",
			Filename: "demo.dem",
		}
		_, err := uc.RequestUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate presigned URL")
	})
}

func TestConfirmUpload_EdgeCases(t *testing.T) {
	ctx := context.Background()
	validMD5 := "d41d8cd98f00b204e9800998ecf8427e"

	t.Run("validation failure", func(t *testing.T) {
		uc := NewUploadUseCase(nil, nil, nil, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: ""}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ports.ErrInvalidUploadRequest.Error())
	})

	t.Run("find match repo error", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			findByIDErr: errors.New("db error"),
		}
		uc := NewUploadUseCase(matchRepo, nil, nil, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find match")
	})

	t.Run("match not found", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: make(map[string]*entities.Match),
		}
		uc := NewUploadUseCase(matchRepo, nil, nil, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ports.ErrMatchNotFound.Error())
	})

	t.Run("match ownership error", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "other-user"},
			},
		}
		uc := NewUploadUseCase(matchRepo, nil, nil, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong to user")
	})

	t.Run("get object info general storage error", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "user-1"},
			},
		}
		storage := &mockStorageService{objInfoErr: errors.New("s3 unavailable")}
		uc := NewUploadUseCase(matchRepo, nil, storage, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify object in storage")
	})

	t.Run("duplicate check error", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "user-1"},
			},
			findByDemoMD5Err: errors.New("db error"),
		}
		storage := &mockStorageService{objInfo: &ports.ObjectInfo{MD5Hash: validMD5}}

		uc := NewUploadUseCase(matchRepo, nil, storage, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}

		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check for duplicate")
	})

	t.Run("duplicate MD5 exists (race guard)", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1":     {ID: "match-1", UserID: "user-1"},
				"match-other": {ID: "match-other", UserID: "user-1"},
			},
			md5Index: map[string]string{
				validMD5: "match-other",
			},
		}

		storage := &mockStorageService{objInfo: &ports.ObjectInfo{MD5Hash: validMD5}}
		uc := NewUploadUseCase(matchRepo, nil, storage, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already uses this MD5")
	})

	t.Run("update match fails", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "user-1"},
			},
			updateErr: errors.New("update failed"),
		}

		storage := &mockStorageService{objInfo: &ports.ObjectInfo{MD5Hash: validMD5}}
		uc := NewUploadUseCase(matchRepo, nil, storage, nil, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update match")
	})

	t.Run("queue publish fails", func(t *testing.T) {
		matchRepo := &mockMatchRepositoryWithErrors{
			matches: map[string]*entities.Match{
				"match-1": {ID: "match-1", UserID: "user-1"},
			},
		}

		storage := &mockStorageService{objInfo: &ports.ObjectInfo{MD5Hash: validMD5}}
		publisher := &mockDemoQueuePublisher{publishErr: errors.New("broker offline")}
		uc := NewUploadUseCase(matchRepo, nil, storage, publisher, "bucket", 15*time.Minute)
		req := &ports.ConfirmUploadRequest{MatchID: "match-1", UserID: "user-1", BucketKey: "key"}
		err := uc.ConfirmUpload(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish job")
	})
}

func TestSanitizePathSegment(t *testing.T) {
	// Test consecutive underscores collapse, trimming, and trailing replacements
	result := sanitizePathSegment("__a__b:c\\d..e//f__")
	// "a_b_c_d_e_f"
	assert.Equal(t, "a_b_c_d_e_f", result)
}

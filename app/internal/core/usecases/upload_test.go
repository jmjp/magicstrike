package usecases

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// --- Mock implementations ---

func ptr(s string) *string {
	return &s
}

type mockStorageService struct {
	presignURL string
	presignErr error
	objInfo    *ports.ObjectInfo
	objInfoErr error
	downloadRC io.ReadCloser
	downloadErr error
}

func (m *mockStorageService) GeneratePresignedUploadURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return m.presignURL, m.presignErr
}

func (m *mockStorageService) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return m.downloadRC, m.downloadErr
}

func (m *mockStorageService) GetObjectInfo(ctx context.Context, key string) (*ports.ObjectInfo, error) {
	return m.objInfo, m.objInfoErr
}

type mockDemoQueuePublisher struct {
	publishErr error
	lastJob    *ports.DemoJobMessage
}

func (m *mockDemoQueuePublisher) PublishDemoJob(ctx context.Context, job *ports.DemoJobMessage) error {
	m.lastJob = job
	return m.publishErr
}

type mockMatchRepository struct {
	matches       map[string]*entities.Match
	md5Index      map[string]string
	saveErr       error
	findErr       error
	updateErr     error
}

func newMockMatchRepo() *mockMatchRepository {
	return &mockMatchRepository{
		matches:  make(map[string]*entities.Match),
		md5Index: make(map[string]string),
	}
}

func (m *mockMatchRepository) Save(ctx context.Context, match *entities.Match) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	cp := *match
	m.matches[match.ID] = &cp
	if match.DemoMD5 != nil && *match.DemoMD5 != "" {
		m.md5Index[*match.DemoMD5] = match.ID
	}
	return nil
}

func (m *mockMatchRepository) FindByID(ctx context.Context, id string) (*entities.Match, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	match, ok := m.matches[id]
	if !ok {
		return nil, nil
	}
	cp := *match
	return &cp, nil
}

func (m *mockMatchRepository) FindByDemoMD5(ctx context.Context, md5Hash string) (*entities.Match, error) {
	if md5Hash == "" {
		return nil, nil
	}
	if m.findErr != nil {
		return nil, m.findErr
	}
	id, ok := m.md5Index[md5Hash]
	if !ok {
		return nil, nil
	}
	return m.FindByID(ctx, id)
}

func (m *mockMatchRepository) Update(ctx context.Context, match *entities.Match) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	cp := *match
	m.matches[match.ID] = &cp
	if match.DemoMD5 != nil && *match.DemoMD5 != "" {
		m.md5Index[*match.DemoMD5] = match.ID
	}
	return nil
}

func (m *mockMatchRepository) List(ctx context.Context, limit, offset int) ([]*entities.Match, error) {
	return nil, nil
}

func (m *mockMatchRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
	return nil, nil
}

type mockUserRepository struct {
	users map[string]*entities.User
}

func newMockUserRepo() *mockUserRepository {
	return &mockUserRepository{users: make(map[string]*entities.User)}
}

func (m *mockUserRepository) Save(ctx context.Context, user *entities.User) error { return nil }
func (m *mockUserRepository) FindByID(ctx context.Context, id string) (*entities.User, error) {
	u, ok := m.users[id]
	if !ok {
		// Return a dummy user for testing
		return &entities.User{Username: "testuser", Email: "test@test.com"}, nil
	}
	return u, nil
}
func (m *mockUserRepository) FindByEmail(ctx context.Context, email string) (*entities.User, error) { return nil, nil }
func (m *mockUserRepository) FindByUsername(ctx context.Context, username string) (*entities.User, error) { return nil, nil }
func (m *mockUserRepository) Update(ctx context.Context, user *entities.User) error { return nil }

// --- Tests ---

func TestRequestUpload_Success(t *testing.T) {
	storage := &mockStorageService{presignURL: "https://minio.local/bucket/key?signature=abc"}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "test-bucket", 15*time.Minute)

	req := &ports.UploadRequest{
		UserID:   "user-123",
		MatchID:  "match-abc",
		Filename: "game.dem",
		TeamA:    "Navi",
		TeamB:    "FaZe",
		MapName:  "de_dust2",
	}

	resp, err := uc.RequestUpload(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadURL != "https://minio.local/bucket/key?signature=abc" {
		t.Errorf("UploadURL: want presigned URL, got %s", resp.UploadURL)
	}
	if resp.MatchID == "" {
		t.Errorf("MatchID should not be empty")
	}
	if !strings.Contains(resp.BucketKey, "user-123") {
		t.Errorf("BucketKey should contain user ID: %s", resp.BucketKey)
	}
	if !strings.Contains(resp.BucketKey, "game.dem") {
		t.Errorf("BucketKey should contain filename: %s", resp.BucketKey)
	}
}

func TestRequestUpload_ValidationError(t *testing.T) {
	storage := &mockStorageService{}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	tests := []struct {
		name    string
		req     *ports.UploadRequest
		wantErr string
	}{
		{
			name:    "empty user ID",
			req:     &ports.UploadRequest{UserID: "", MatchID: "m1", Filename: "test.dem", TeamA: "A", TeamB: "B", MapName: "de_dust2"},
			wantErr: "user ID is required",
		},
		{
			name:    "empty filename",
			req:     &ports.UploadRequest{UserID: "u1", MatchID: "m1", Filename: "", TeamA: "A", TeamB: "B", MapName: "de_dust2"},
			wantErr: "filename is required",
		},
		{
			name:    "invalid extension",
			req:     &ports.UploadRequest{UserID: "u1", MatchID: "m1", Filename: "test.txt", TeamA: "A", TeamB: "B", MapName: "de_dust2"},
			wantErr: ".dem extension",
		},
		{
			name:    "same team names",
			req:     &ports.UploadRequest{UserID: "u1", MatchID: "m1", Filename: "test.dem", TeamA: "Navi", TeamB: "Navi", MapName: "de_dust2"},
			wantErr: "cannot be the same",
		},

		{
			name:    "match ID too long",
			req:     &ports.UploadRequest{UserID: "u1", MatchID: strings.Repeat("x", 65), Filename: "test.dem", TeamA: "A", TeamB: "B", MapName: "de_dust2"},
			wantErr: "at most 64 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.RequestUpload(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error should contain %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestRequestUpload_DuplicateMD5(t *testing.T) {
	storage := &mockStorageService{presignURL: "https://example.com/url"}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	// Pre-create a match with the same MD5
	md5 := "abc123def4567890abc123def4567890"
	mapName := entities.MapNameDust2
	existing, err := entities.NewMatch("user1", ptr("TeamX"), ptr("TeamY"), &md5, &mapName)
	if err != nil {
		t.Fatalf("NewMatch failed: %v", err)
	}
	matchRepo.Save(context.Background(), existing)

	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	req := &ports.UploadRequest{
		UserID:   "user-1",
		MatchID:  "new-match",
		Filename: "game.dem",
		MD5Hash:  &md5,
		TeamA:    "A",
		TeamB:    "B",
		MapName:  "de_dust2",
	}

	_, err = uc.RequestUpload(context.Background(), req)
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
	if !strings.Contains(err.Error(), "already uses this MD5") {
		t.Errorf("error should mention MD5 collision, got: %v", err)
	}
}

func TestConfirmUpload_Success(t *testing.T) {
	md5Hash := "abc123def4567890abc123def4567890"
	storage := &mockStorageService{
		objInfo: &ports.ObjectInfo{
			Key:     "uploads/user1/match1/game.dem",
			Size:    1024000,
			MD5Hash: md5Hash,
		},
	}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	mapName := entities.MapNameDust2
	match, err := entities.NewMatch("user1", ptr("TeamA"), ptr("TeamB"), nil, &mapName)
	if err != nil {
		t.Fatalf("NewMatch failed: %v", err)
	}
	match.ID = "match-001"
	matchRepo.Save(context.Background(), match)

	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err = uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "match-001",
		BucketKey: "uploads/user1/match1/game.dem",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify match was updated with MD5
	updated, _ := matchRepo.FindByID(context.Background(), "match-001")
	if updated.DemoMD5 == nil || *updated.DemoMD5 != md5Hash {
		t.Errorf("Match DemoMD5 should be %s, got %v", md5Hash, updated.DemoMD5)
	}

	// Verify job was published
	if queue.lastJob == nil {
		t.Fatal("expected job to be published")
	}
	if queue.lastJob.MatchID != "match-001" {
		t.Errorf("job MatchID: want match-001, got %s", queue.lastJob.MatchID)
	}
	if queue.lastJob.MD5Hash != md5Hash {
		t.Errorf("job MD5Hash: want %s, got %s", md5Hash, queue.lastJob.MD5Hash)
	}
}

func TestConfirmUpload_MatchNotFound(t *testing.T) {
	storage := &mockStorageService{}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err := uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "nonexistent",
		BucketKey: "path/file.dem",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent match")
	}
	if !strings.Contains(err.Error(), "match not found") {
		t.Errorf("expected 'match not found', got: %v", err)
	}
}

func TestConfirmUpload_ObjectNotFound(t *testing.T) {
	storage := &mockStorageService{
		objInfoErr: ports.ErrObjectNotFound,
	}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	mapName := entities.MapNameDust2
	match, err := entities.NewMatch("user1", ptr("A"), ptr("B"), nil, &mapName)
	if err != nil {
		t.Fatalf("NewMatch failed: %v", err)
	}
	match.ID = "match-001"
	matchRepo.Save(context.Background(), match)

	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err = uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "match-001",
		BucketKey: "missing/file.dem",
	})
	if err == nil {
		t.Fatal("expected error for missing object")
	}
	if !strings.Contains(err.Error(), "not been uploaded") {
		t.Errorf("expected 'not been uploaded', got: %v", err)
	}
}

func TestConfirmUpload_PathTraversalRejected(t *testing.T) {
	storage := &mockStorageService{}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err := uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "match-001",
		BucketKey: "../etc/passwd",
	})
	if err == nil {
		t.Fatal("expected validation error for path traversal")
	}
	if !strings.Contains(err.Error(), "invalid path segments") {
		t.Errorf("expected path traversal rejection, got: %v", err)
	}
}

func TestConfirmUpload_DuplicateMD5_DifferentMatch(t *testing.T) {
	md5 := "aaa11122233344455566677788899900" // 32-char hex
	storage := &mockStorageService{
		objInfo: &ports.ObjectInfo{MD5Hash: md5},
	}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	// Create original match with MD5
	mapName := entities.MapNameInferno
	m1, err := entities.NewMatch("user1", ptr("A"), ptr("B"), &md5, &mapName)
	if err != nil {
		t.Fatalf("NewMatch m1 failed: %v", err)
	}
	m1.ID = "match-original"
	matchRepo.Save(context.Background(), m1)

	// Create second match WITHOUT MD5 (will attempt to claim same MD5)
	m2, err := entities.NewMatch("user1", ptr("C"), ptr("D"), nil, &mapName)
	if err != nil {
		t.Fatalf("NewMatch m2 failed: %v", err)
	}
	m2.ID = "match-second"
	matchRepo.Save(context.Background(), m2)

	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err = uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "match-second",
		BucketKey: "path/game.dem",
	})
	if err == nil {
		t.Fatal("expected duplicate MD5 error")
	}
	if !strings.Contains(err.Error(), "already uses this MD5") {
		t.Errorf("expected duplicate MD5 error, got: %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"game.dem", "game.dem"},
		{"path/to/game.dem", "path_to_game.dem"},
		{"back\\slash.dem", "back_slash.dem"},
		{"../../../etc/passwd", "______etc_passwd"},
		{"null\x00byte.dem", "nullbyte.dem"},
		{"..hidden.dem", "_hidden.dem"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q): want %q, got %q", tt.input, tt.expected, result)
			}
		})
	}
}

func TestUploadUseCase_PresignTTL_Default(t *testing.T) {
	storage := &mockStorageService{presignURL: "https://example.com/url"}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	// Zero TTL should default to 15 minutes
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 0)
	uploadUC := uc.(*UploadUseCase)
	if uploadUC.presignTTL != DefaultPresignTTL {
		t.Errorf("zero TTL should default to %v, got %v", DefaultPresignTTL, uploadUC.presignTTL)
	}

	// Negative TTL should also default
	uc2 := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", -1*time.Second)
	uploadUC2 := uc2.(*UploadUseCase)
	if uploadUC2.presignTTL != DefaultPresignTTL {
		t.Errorf("negative TTL should default to %v, got %v", DefaultPresignTTL, uploadUC2.presignTTL)
	}
}

func TestRequestUpload_PresignFailure(t *testing.T) {
	storage := &mockStorageService{presignErr: ports.ErrStorageUnavailable}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	_, err := uc.RequestUpload(context.Background(), &ports.UploadRequest{
		UserID: "u1", MatchID: "m1", Filename: "test.dem", TeamA: "A", TeamB: "B", MapName: "de_dust2",
	})
	if err == nil {
		t.Fatal("expected error when presigned URL generation fails")
	}
	// Match should still have been saved
	var savedMatch *entities.Match
	for _, matchVal := range matchRepo.matches {
		savedMatch = matchVal
		break
	}
	if savedMatch == nil {
		t.Error("match should have been saved even though presign failed")
	}
}

func TestRequestUpload_InvalidMD5Hash(t *testing.T) {
	storage := &mockStorageService{}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	invalidMD5 := "not-a-valid-md5"
	_, err := uc.RequestUpload(context.Background(), &ports.UploadRequest{
		UserID: "u1", MatchID: "m1", Filename: "test.dem", MD5Hash: &invalidMD5,
		TeamA: "A", TeamB: "B", MapName: "de_dust2",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid MD5")
	}
	if !strings.Contains(err.Error(), "32-character hex") {
		t.Errorf("expected '32-character hex' error, got: %v", err)
	}
}

func TestConfirmUpload_FindByIDError(t *testing.T) {
	storage := &mockStorageService{}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	matchRepo.findErr = errors.New("db connection failed")
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err := uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "match-001",
		BucketKey: "path/file.dem",
	})
	if err == nil {
		t.Fatal("expected error when FindByID fails")
	}
	if !strings.Contains(err.Error(), "failed to find match") {
		t.Errorf("expected 'failed to find match', got: %v", err)
	}
}

func TestConfirmUpload_PublishError(t *testing.T) {
	md5Hash := "abc123def4567890abc123def4567890"
	storage := &mockStorageService{
		objInfo: &ports.ObjectInfo{
			Key:     "uploads/game.dem",
			MD5Hash: md5Hash,
		},
	}
	queue := &mockDemoQueuePublisher{publishErr: ports.ErrQueueUnavailable}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()

	mapName := entities.MapNameDust2
	match, err := entities.NewMatch("user1", ptr("A"), ptr("B"), nil, &mapName)
	if err != nil {
		t.Fatalf("NewMatch failed: %v", err)
	}
	match.ID = "match-001"
	matchRepo.Save(context.Background(), match)

	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	err = uc.ConfirmUpload(context.Background(), &ports.ConfirmUploadRequest{
		UserID:  "user1",
		MatchID:   "match-001",
		BucketKey: "uploads/game.dem",
	})
	if err == nil {
		t.Fatal("expected error when queue publish fails")
	}
	if !strings.Contains(err.Error(), "failed to publish job") {
		t.Errorf("expected 'failed to publish job', got: %v", err)
	}
}

func TestConfirmUpload_ValidationError(t *testing.T) {
	storage := &mockStorageService{}
	queue := &mockDemoQueuePublisher{}
	matchRepo := newMockMatchRepo()
	userRepo := newMockUserRepo()
	uc := NewUploadUseCase(matchRepo, userRepo, storage, queue, "bucket", 15*time.Minute)

	tests := []struct {
		name    string
		req     *ports.ConfirmUploadRequest
		wantErr string
	}{
		{"empty match ID", &ports.ConfirmUploadRequest{UserID: "user1", MatchID: "", BucketKey: "path"}, "match ID is required"},
		{"empty bucket key", &ports.ConfirmUploadRequest{UserID: "user1", MatchID: "m1", BucketKey: ""}, "bucket key is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := uc.ConfirmUpload(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

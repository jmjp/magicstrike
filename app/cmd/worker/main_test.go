package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

type stubStorageService struct {
	downloadFn func(ctx context.Context, path string) (io.ReadCloser, error)
}

func (s *stubStorageService) Download(ctx context.Context, path string) (io.ReadCloser, error) {
	if s.downloadFn != nil {
		return s.downloadFn(ctx, path)
	}
	return io.NopCloser(bytes.NewReader([]byte("dummy content"))), nil
}

func (s *stubStorageService) GeneratePresignedUploadURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	return "", nil
}

func (s *stubStorageService) GetObjectInfo(ctx context.Context, key string) (*ports.ObjectInfo, error) {
	return &ports.ObjectInfo{
		Key:  key,
		Size: 100,
	}, nil
}

type stubMatchRepository struct {
	findByDemoMD5Fn func(ctx context.Context, md5 string) (*entities.Match, error)
	findByIDFn      func(ctx context.Context, id string) (*entities.Match, error)
}

func (m *stubMatchRepository) Save(ctx context.Context, match *entities.Match) error {
	return nil
}

func (m *stubMatchRepository) Update(ctx context.Context, match *entities.Match) error {
	return nil
}

func (m *stubMatchRepository) FindByID(ctx context.Context, id string) (*entities.Match, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *stubMatchRepository) FindByDemoMD5(ctx context.Context, md5 string) (*entities.Match, error) {
	if m.findByDemoMD5Fn != nil {
		return m.findByDemoMD5Fn(ctx, md5)
	}
	return nil, nil
}

func (m *stubMatchRepository) List(ctx context.Context, limit, offset int) ([]*entities.Match, error) {
	return nil, nil
}

func (m *stubMatchRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*entities.Match, error) {
	return nil, nil
}

type stubIngestionUseCase struct {
	ingestDemoFn func(ctx context.Context, matchID string, r io.Reader) error
}

func (i *stubIngestionUseCase) IngestDemo(ctx context.Context, matchID string, r io.Reader) error {
	if i.ingestDemoFn != nil {
		return i.ingestDemoFn(ctx, matchID, r)
	}
	return nil
}

func TestEnvOr(t *testing.T) {
	os.Setenv("TEST_ENV_OR_KEY", "value")
	defer os.Unsetenv("TEST_ENV_OR_KEY")

	if val := envOr("TEST_ENV_OR_KEY", "fallback"); val != "value" {
		t.Errorf("expected 'value', got %q", val)
	}

	if val := envOr("NON_EXISTENT_KEY", "fallback"); val != "fallback" {
		t.Errorf("expected 'fallback', got %q", val)
	}
}

func TestGetMaxFileSize(t *testing.T) {
	if val := getMaxFileSize(); val != int64(900*1024*1024) {
		t.Errorf("expected default 900MB, got %d", val)
	}

	os.Setenv("MAX_DEMO_FILE_SIZE", "50")
	defer os.Unsetenv("MAX_DEMO_FILE_SIZE")

	if val := getMaxFileSize(); val != 50 {
		t.Errorf("expected 50, got %d", val)
	}

	os.Setenv("MAX_DEMO_FILE_SIZE", "invalid")
	if val := getMaxFileSize(); val != int64(900*1024*1024) {
		t.Errorf("expected default 900MB, got %d", val)
	}
}

func TestProcessJob(t *testing.T) {
	content := []byte("magic strike demo content")
	hash := md5.Sum(content)
	contentMD5 := hex.EncodeToString(hash[:])

	t.Run("success", func(t *testing.T) {
		storage := &stubStorageService{
			downloadFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(content)), nil
			},
		}
		repo := &stubMatchRepository{}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID:    "match-1",
			BucketPath: "demos/match-1.dem",
			MD5Hash:    contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("dedup guard - duplicate by MD5", func(t *testing.T) {
		storage := &stubStorageService{}
		repo := &stubMatchRepository{
			findByDemoMD5Fn: func(ctx context.Context, md5 string) (*entities.Match, error) {
				// return a match with a different ID (meaning same file already processed elsewhere)
				return &entities.Match{ID: "other-match-id"}, nil
			},
		}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err != nil {
			t.Errorf("expected no error (just skip), got %v", err)
		}
	})

	t.Run("dedup guard - DB error", func(t *testing.T) {
		storage := &stubStorageService{}
		repo := &stubMatchRepository{
			findByDemoMD5Fn: func(ctx context.Context, md5 string) (*entities.Match, error) {
				return nil, errors.New("db error")
			},
		}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err == nil {
			t.Error("expected db error to propagate")
		}
	})

	t.Run("already processed - finished state", func(t *testing.T) {
		storage := &stubStorageService{}
		repo := &stubMatchRepository{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return &entities.Match{ID: "match-1", Status: "finished"}, nil
			},
		}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err != nil {
			t.Errorf("expected no error (just skip), got %v", err)
		}
	})

	t.Run("already processed - DB error", func(t *testing.T) {
		storage := &stubStorageService{}
		repo := &stubMatchRepository{
			findByIDFn: func(ctx context.Context, id string) (*entities.Match, error) {
				return nil, errors.New("db error")
			},
		}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err == nil {
			t.Error("expected error to propagate")
		}
	})

	t.Run("download error", func(t *testing.T) {
		storage := &stubStorageService{
			downloadFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
				return nil, errors.New("download failed")
			},
		}
		repo := &stubMatchRepository{}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err == nil {
			t.Error("expected download error to propagate")
		}
	})

	t.Run("file exceeds maximum size", func(t *testing.T) {
		os.Setenv("MAX_DEMO_FILE_SIZE", "5")
		defer os.Unsetenv("MAX_DEMO_FILE_SIZE")

		storage := &stubStorageService{
			downloadFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader([]byte("too long content"))), nil
			},
		}
		repo := &stubMatchRepository{}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte("exceeds maximum size")) {
			t.Errorf("expected size limit error, got %v", err)
		}
	})

	t.Run("MD5 mismatch", func(t *testing.T) {
		storage := &stubStorageService{
			downloadFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(content)), nil
			},
		}
		repo := &stubMatchRepository{}
		ingest := &stubIngestionUseCase{}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: "wrong-md5",
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte("MD5 mismatch")) {
			t.Errorf("expected MD5 mismatch error, got %v", err)
		}
	})

	t.Run("ingestion failed", func(t *testing.T) {
		storage := &stubStorageService{
			downloadFn: func(ctx context.Context, path string) (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(content)), nil
			},
		}
		repo := &stubMatchRepository{}
		ingest := &stubIngestionUseCase{
			ingestDemoFn: func(ctx context.Context, matchID string, r io.Reader) error {
				return errors.New("parse failed")
			},
		}

		job := &ports.DemoJobMessage{
			MatchID: "match-1",
			MD5Hash: contentMD5,
		}

		err := processJob(context.Background(), job, storage, repo, ingest)
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte("ingestion failed")) {
			t.Errorf("expected ingestion error, got %v", err)
		}
	})
}

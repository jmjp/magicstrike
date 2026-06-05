package main

import (
	"context"
	"os"
	"testing"
)

func TestBuildMatchRepo_MemoryFallback(t *testing.T) {
	origHost := os.Getenv("POSTGRES_HOST")
	os.Unsetenv("POSTGRES_HOST")
	defer func() {
		if origHost != "" {
			os.Setenv("POSTGRES_HOST", origHost)
		}
	}()

	ctx := context.Background()
	repo := buildMatchRepo(ctx)

	if repo == nil {
		t.Fatal("expected non-nil match repo")
	}

	// Should work with basic operations in memory
	err := repo.Save(ctx, nil)
	if err != nil {
		t.Errorf("Save(nil) should not error, got: %v", err)
	}
}

func TestBuildMatchRepo_Postgres(t *testing.T) {
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		t.Skip("POSTGRES_HOST not set, skipping postgres buildMatchRepo test")
	}

	ctx := context.Background()
	repo := buildMatchRepo(ctx)
	if repo == nil {
		t.Fatal("expected non-nil match repo from postgres")
	}
}

func TestEnvOr_Worker(t *testing.T) {
	tests := []struct {
		key      string
		fallback string
		setVal   string
		want     string
	}{
		{"NONEXISTENT_KEY_XYZ", "default", "", "default"},
		{"SET_KEY_ABC", "fallback", "actual", "actual"},
	}

	for _, tt := range tests {
		if tt.setVal != "" {
			os.Setenv(tt.key, tt.setVal)
			defer os.Unsetenv(tt.key)
		}
		got := envOr(tt.key, tt.fallback)
		if got != tt.want {
			t.Errorf("envOr(%q, %q) = %q, want %q", tt.key, tt.fallback, got, tt.want)
		}
	}
}

func TestGetMaxFileSize_Worker(t *testing.T) {
	t.Run("default size", func(t *testing.T) {
		os.Unsetenv("MAX_DEMO_FILE_SIZE")
		size := getMaxFileSize()
		expected := int64(900 * 1024 * 1024)
		if size != expected {
			t.Errorf("expected %d, got %d", expected, size)
		}
	})

	t.Run("custom size", func(t *testing.T) {
		os.Setenv("MAX_DEMO_FILE_SIZE", "1048576")
		defer os.Unsetenv("MAX_DEMO_FILE_SIZE")
		size := getMaxFileSize()
		expected := int64(1048576)
		if size != expected {
			t.Errorf("expected %d, got %d", expected, size)
		}
	})

	t.Run("invalid size falls back to default", func(t *testing.T) {
		os.Setenv("MAX_DEMO_FILE_SIZE", "invalid")
		defer os.Unsetenv("MAX_DEMO_FILE_SIZE")
		size := getMaxFileSize()
		expected := int64(900 * 1024 * 1024)
		if size != expected {
			t.Errorf("expected %d, got %d", expected, size)
		}
	})

	t.Run("zero size falls back to default", func(t *testing.T) {
		os.Setenv("MAX_DEMO_FILE_SIZE", "0")
		defer os.Unsetenv("MAX_DEMO_FILE_SIZE")
		size := getMaxFileSize()
		expected := int64(900 * 1024 * 1024)
		if size != expected {
			t.Errorf("expected %d, got %d", expected, size)
		}
	})
}

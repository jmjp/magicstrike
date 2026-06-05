package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"magicstrike/internal/core/entities"
	"magicstrike/internal/core/ports"
)

// DefaultPresignTTL is the default expiration time for presigned upload URLs.
const DefaultPresignTTL = 15 * 60 * time.Second

// UploadUseCase implements ports.UploadUseCase.
// It orchestrates the upload request → presigned URL generation → upload confirmation → queue publishing flow.
type UploadUseCase struct {
	matchRepo  ports.MatchRepository
	userRepo   ports.UserRepository
	storageSvc ports.StorageService
	queuePub   ports.DemoQueuePublisher
	bucketName string
	presignTTL time.Duration
}

// NewUploadUseCase creates a new UploadUseCase with the given dependencies.
// If presignTTL is zero or negative, DefaultPresignTTL is used.
func NewUploadUseCase(
	matchRepo ports.MatchRepository,
	userRepo ports.UserRepository,
	storageSvc ports.StorageService,
	queuePub ports.DemoQueuePublisher,
	bucketName string,
	presignTTL time.Duration,
) ports.UploadUseCase {
	if presignTTL <= 0 {
		presignTTL = DefaultPresignTTL
	}
	return &UploadUseCase{
		matchRepo:  matchRepo,
		userRepo:   userRepo,
		storageSvc: storageSvc,
		queuePub:   queuePub,
		bucketName: bucketName,
		presignTTL: presignTTL,
	}
}

// RequestUpload implements ports.UploadUseCase.
func (uc *UploadUseCase) RequestUpload(ctx context.Context, input *ports.UploadRequest) (*ports.UploadResponse, error) {
	// 1. Validate the request
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ports.ErrInvalidUploadRequest, err)
	}

	// 2. Verify user exists
	user, err := uc.userRepo.FindByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", input.UserID)
	}

	// 3. Check for duplicate MD5 if provided
	if input.MD5Hash != nil {
		existing, err := uc.matchRepo.FindByDemoMD5(ctx, *input.MD5Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to check for duplicate: %w", err)
		}
		if existing != nil {
			return nil, fmt.Errorf("%w: match %s already uses this MD5", ports.ErrDuplicateUpload, existing.ID)
		}
	}

	// 4. Check if match already exists with this ID (ownership guard)
	if input.MatchID != "" {
		existingMatch, err := uc.matchRepo.FindByID(ctx, input.MatchID)
		if err != nil {
			return nil, fmt.Errorf("failed to check for existing match: %w", err)
		}
		if existingMatch != nil {
			if existingMatch.UserID != input.UserID {
				return nil, fmt.Errorf("%w: match %s belongs to another user", ports.ErrInvalidUploadRequest, input.MatchID)
			}
			// Re-requesting for the same match by the same user — return existing info
			bucketKey := fmt.Sprintf("uploads/%s/%s/%s",
				sanitizePathSegment(input.UserID),
				sanitizePathSegment(existingMatch.ID),
				sanitizeFilename(input.Filename))
			uploadURL, err := uc.storageSvc.GeneratePresignedUploadURL(ctx, bucketKey, uc.presignTTL)
			if err != nil {
				return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
			}
			return &ports.UploadResponse{
				UploadURL: uploadURL,
				BucketKey: bucketKey,
				ExpiresAt: time.Now().Add(uc.presignTTL),
				MatchID:   existingMatch.ID,
			}, nil
		}
	}

	// 5. Create the Match entity in "waiting" state
	var teamA *string
	if input.TeamA != "" {
		teamA = &input.TeamA
	}
	var teamB *string
	if input.TeamB != "" {
		teamB = &input.TeamB
	}
	var mapName *entities.MapName
	if input.MapName != "" {
		mn := entities.MapName(input.MapName)
		mapName = &mn
	}

	match, err := entities.NewMatch(input.UserID, teamA, teamB, input.MD5Hash, mapName)
	if err != nil {
		return nil, fmt.Errorf("failed to create match: %w", err)
	}

	if err := uc.matchRepo.Save(ctx, match); err != nil {
		return nil, fmt.Errorf("failed to save match: %w", err)
	}

	// 5. Generate the bucket key: uploads/{userID}/{matchID}/{filename}
	// All path segments are sanitized to prevent path traversal attacks.
	bucketKey := fmt.Sprintf("uploads/%s/%s/%s",
		sanitizePathSegment(input.UserID),
		sanitizePathSegment(match.ID),
		sanitizeFilename(input.Filename))

	// 6. Generate the presigned upload URL
	uploadURL, err := uc.storageSvc.GeneratePresignedUploadURL(ctx, bucketKey, uc.presignTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return &ports.UploadResponse{
		UploadURL: uploadURL,
		BucketKey: bucketKey,
		ExpiresAt: time.Now().Add(uc.presignTTL),
		MatchID:   match.ID,
	}, nil
}

// ConfirmUpload implements ports.UploadUseCase.
func (uc *UploadUseCase) ConfirmUpload(ctx context.Context, input *ports.ConfirmUploadRequest) error {
	// 1. Validate the request
	if err := input.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ports.ErrInvalidUploadRequest, err)
	}

	// 2. Find the match
	match, err := uc.matchRepo.FindByID(ctx, input.MatchID)
	if err != nil {
		return fmt.Errorf("failed to find match: %w", err)
	}
	if match == nil {
		return fmt.Errorf("%w: %s", ports.ErrMatchNotFound, input.MatchID)
	}

	// 2a. Verify ownership — only the match owner can confirm the upload
	if match.UserID != input.UserID {
		return fmt.Errorf("%w: match %s does not belong to user %s", ports.ErrInvalidUploadRequest, input.MatchID, input.UserID)
	}

	// 3. Verify the object exists in storage and get its metadata
	objInfo, err := uc.storageSvc.GetObjectInfo(ctx, input.BucketKey)
	if err != nil {
		if errors.Is(err, ports.ErrObjectNotFound) {
			return fmt.Errorf("%w: file has not been uploaded yet at %s", ports.ErrObjectNotFound, input.BucketKey)
		}
		return fmt.Errorf("failed to verify object in storage: %w", err)
	}

	// 4. Update match with the storage MD5 hash
	md5Hash := objInfo.MD5Hash
	match.DemoMD5 = &md5Hash

	// 5. Check for duplicate MD5 (defense against race condition)
	existing, err := uc.matchRepo.FindByDemoMD5(ctx, md5Hash)
	if err != nil {
		return fmt.Errorf("failed to check for duplicate: %w", err)
	}
	if existing != nil && existing.ID != match.ID {
		return fmt.Errorf("%w: match %s already uses this MD5", ports.ErrDuplicateUpload, existing.ID)
	}

	if err := uc.matchRepo.Update(ctx, match); err != nil {
		return fmt.Errorf("failed to update match: %w", err)
	}

	// 6. Publish the demo processing job to the queue
	job, err := entities.NewDemoJobMessage(match.ID, input.BucketKey, md5Hash, time.Now())
	if err != nil {
		return fmt.Errorf("failed to create job message: %w", err)
	}

	// Convert entity to port type for the publisher
	queueJob := &ports.DemoJobMessage{
		MatchID:    job.MatchID,
		BucketPath: job.BucketPath,
		MD5Hash:    job.MD5Hash,
		UploadedAt: job.UploadedAt,
	}

	if err := uc.queuePub.PublishDemoJob(ctx, queueJob); err != nil {
		return fmt.Errorf("failed to publish job: %w", err)
	}

	return nil
}

// sanitizeFilename removes path separators and special characters from a filename.
func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.ReplaceAll(name, "\x00", "")
	return name
}

// sanitizePathSegment removes characters that could enable path traversal or injection
// in a bucket key segment. It is more aggressive than sanitizeFilename because
// bucket key segments form a storage hierarchy.
func sanitizePathSegment(segment string) string {
	segment = strings.ReplaceAll(segment, "/", "_")
	segment = strings.ReplaceAll(segment, "\\", "_")
	segment = strings.ReplaceAll(segment, "..", "_")
	segment = strings.ReplaceAll(segment, "\x00", "")
	segment = strings.ReplaceAll(segment, ":", "_")
	// Collapse consecutive underscores
	for strings.Contains(segment, "__") {
		segment = strings.ReplaceAll(segment, "__", "_")
	}
	return strings.Trim(segment, "_")
}

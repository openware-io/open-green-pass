// Package application 测试证据元数据服务（校验 + Trusted 写）。
package application

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/internal/trusted/domain"
	gperr "github.com/openware-io/open-green-pass/pkg/errors"
	"github.com/openware-io/open-green-pass/pkg/id"
)

var allowedEvidenceMIME = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true,
	"video/mp4": true, "video/webm": true,
	"application/json": true, "text/plain": true, "text/csv": true,
}

// EvidenceService 证据元数据应用服务。
type EvidenceObjectStore interface {
	PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	URI(key string) string
}

type EvidenceService struct {
	repo  domain.EvidenceRepository
	gen   *id.Generator
	store EvidenceObjectStore
}

func NewEvidenceService(repo domain.EvidenceRepository, gen *id.Generator) *EvidenceService {
	return &EvidenceService{repo: repo, gen: gen}
}

func (s *EvidenceService) SetObjectStore(store EvidenceObjectStore) { s.store = store }

type PresignEvidenceInput struct {
	Kind      string
	MimeType  string
	SizeBytes int64
	SHA256    string
}

type PresignEvidenceResult struct {
	UploadURL  string    `json:"upload_url"`
	StorageURI string    `json:"storage_uri"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// PresignUpload creates a short-lived PUT URL and the immutable object URI.
func (s *EvidenceService) PresignUpload(ctx context.Context, findingID int64, in PresignEvidenceInput) (*PresignEvidenceResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if s.store == nil {
		return nil, errors.New("evidence object store is not configured")
	}
	if findingID <= 0 {
		return nil, gperr.Validation("finding_id required")
	}
	if !validEvidenceKind(strings.TrimSpace(in.Kind)) {
		return nil, gperr.Validation("invalid evidence kind")
	}
	if !allowedEvidenceMIME[in.MimeType] {
		return nil, gperr.Validation("unsupported evidence mime type")
	}
	limit := int64(10 << 20)
	if strings.HasPrefix(in.MimeType, "video/") {
		limit = 100 << 20
	}
	if in.SizeBytes <= 0 || in.SizeBytes > limit {
		return nil, gperr.Validation("evidence size is invalid")
	}
	if len(in.SHA256) != 64 {
		return nil, gperr.Validation("sha256 must be 64 hex characters")
	}
	if _, err := hex.DecodeString(in.SHA256); err != nil {
		return nil, gperr.Validation("sha256 must be hex")
	}
	key := fmt.Sprintf("%d/%d/%d-%s", teamID, findingID, s.gen.Next(), in.SHA256[:12])
	url, err := s.store.PresignPut(ctx, key, 15*time.Minute)
	if err != nil {
		return nil, err
	}
	return &PresignEvidenceResult{UploadURL: url, StorageURI: s.store.URI(key), ExpiresAt: time.Now().UTC().Add(15 * time.Minute)}, nil
}

// DownloadURL returns a short-lived GET URL for a registered s3:// evidence URI.
func (s *EvidenceService) DownloadURL(ctx context.Context, findingID, evidenceID int64) (string, error) {
	if s.store == nil {
		return "", errors.New("evidence object store is not configured")
	}
	e, err := s.Get(ctx, findingID, evidenceID)
	if err != nil {
		return "", err
	}
	const prefix = "s3://"
	if !strings.HasPrefix(e.StorageURI, prefix) {
		return "", gperr.Validation("evidence is not stored in s3")
	}
	rest := strings.TrimPrefix(e.StorageURI, prefix)
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 || slash == len(rest)-1 {
		return "", gperr.Validation("invalid evidence storage uri")
	}
	return s.store.PresignGet(ctx, rest[slash+1:], 15*time.Minute)
}

// CreateEvidenceInput 证据登记输入（文件已由调用方上传到对象存储）。
type CreateEvidenceInput struct {
	Kind         string
	StorageURI   string
	MimeType     string
	SizeBytes    int64
	Width        *int
	Height       *int
	DurationMS   *int
	SHA256       string
	ThumbnailURI *string
	Redacted     bool
}

// Create 登记一条不可变证据元数据。
func (s *EvidenceService) Create(ctx context.Context, findingID int64, in CreateEvidenceInput) (*domain.Evidence, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	if findingID <= 0 {
		return nil, gperr.Validation("finding_id required")
	}
	in.Kind = strings.TrimSpace(in.Kind)
	if !validEvidenceKind(in.Kind) {
		return nil, gperr.Validation("invalid evidence kind")
	}
	if strings.TrimSpace(in.StorageURI) == "" {
		return nil, gperr.Validation("storage_uri required")
	}
	if !allowedEvidenceMIME[in.MimeType] {
		return nil, gperr.Validation("unsupported evidence mime type")
	}
	if in.SizeBytes <= 0 {
		return nil, gperr.Validation("size_bytes must be positive")
	}
	limit := int64(10 << 20)
	if strings.HasPrefix(in.MimeType, "video/") {
		limit = 100 << 20
	}
	if in.SizeBytes > limit {
		return nil, gperr.Validation("evidence size exceeds limit")
	}
	if len(in.SHA256) != 64 {
		return nil, gperr.Validation("sha256 must be 64 hex characters")
	}
	if _, err := hex.DecodeString(in.SHA256); err != nil {
		return nil, gperr.Validation("sha256 must be hex")
	}
	actor, _ := rls.UserFrom(ctx)
	e := &domain.Evidence{
		ID: s.gen.Next(), TeamID: teamID, FindingID: findingID, Kind: in.Kind,
		StorageURI: in.StorageURI, MimeType: in.MimeType, SizeBytes: in.SizeBytes,
		Width: in.Width, Height: in.Height, DurationMS: in.DurationMS, SHA256: in.SHA256,
		ThumbnailURI: in.ThumbnailURI, Redacted: in.Redacted, CreatedBy: actor,
	}
	if err := s.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// List 查询 finding 证据。
func (s *EvidenceService) List(ctx context.Context, findingID int64) ([]*domain.Evidence, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	return s.repo.List(ctx, teamID, findingID)
}

// Get 查询单条证据。
func (s *EvidenceService) Get(ctx context.Context, findingID, evidenceID int64) (*domain.Evidence, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, gperr.Unauthorized("tenant required")
	}
	e, err := s.repo.Find(ctx, teamID, findingID, evidenceID)
	if err != nil {
		return nil, gperr.NotFound("evidence not found")
	}
	return e, nil
}

func validEvidenceKind(kind string) bool {
	switch kind {
	case "request", "response", "log", "trace", "screenshot", "video":
		return true
	}
	return false
}

package file

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/iamarpitzala/acareca/internal/shared/upload"
)

const (
	uploadWorkerInterval = 30 * time.Second

	expiredWorkerInterval = 5 * time.Minute

	uploadWorkerBatchSize = 50
)

type UploadWorker struct {
	repo    Repository
	storage upload.StorageProvider
}

func NewUploadWorker(repo Repository, storage upload.StorageProvider) *UploadWorker {
	return &UploadWorker{
		repo:    repo,
		storage: storage,
	}
}

func (w *UploadWorker) Start(ctx context.Context) {
	log.Println("[file worker] started")

	verifyTicker := time.NewTicker(uploadWorkerInterval)
	expiredTicker := time.NewTicker(expiredWorkerInterval)
	defer verifyTicker.Stop()
	defer expiredTicker.Stop()

	w.processPending(ctx)
	w.processExpired(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[file worker] stopped")
			return
		case <-verifyTicker.C:
			w.processPending(ctx)
		case <-expiredTicker.C:
			w.processExpired(ctx)
		}
	}
}

func (w *UploadWorker) processPending(ctx context.Context) {
	docs, err := w.repo.FindPendingUploads(ctx, uploadWorkerBatchSize)
	if err != nil {
		log.Printf("[file worker] error fetching pending uploads: %v", err)
		return
	}

	if len(docs) == 0 {
		return
	}

	log.Printf("[file worker] verifying %d pending upload(s)", len(docs))

	presignedProvider, hasPresigned := w.storage.(upload.PresignedURLProvider)

	for _, doc := range docs {
		if err := w.verifyAndUpdate(ctx, doc, presignedProvider, hasPresigned); err != nil {
			log.Printf("[file worker] error processing document %s: %v", doc.ID, err)
		}
	}
}

func (w *UploadWorker) processExpired(ctx context.Context) {
	docs, err := w.repo.FindExpiredPendingUploads(ctx, uploadWorkerBatchSize)
	if err != nil {
		log.Printf("[file worker] error fetching expired uploads: %v", err)
		return
	}

	if len(docs) == 0 {
		return
	}

	log.Printf("[file worker] marking %d expired upload(s) as failed", len(docs))

	for _, doc := range docs {
		if err := w.repo.UpdateStatus(ctx, doc.ID, StatusFailed, &doc, nil); err != nil {
			log.Printf("[file worker] failed to mark document %s as failed: %v", doc.ID, err)
			continue
		}
		log.Printf("[file worker] document %s marked as failed (upload expired at %v)", doc.ID, doc.UploadExpiresAt)
	}
}

func (w *UploadWorker) verifyAndUpdate(ctx context.Context, doc Document, presignedProvider upload.PresignedURLProvider, hasPresigned bool) error {
	exists, size, err := w.objectExists(ctx, doc.ObjectKey, presignedProvider, hasPresigned)
	if err != nil {
		return fmt.Errorf("check object existence for %s: %w", doc.ObjectKey, err)
	}

	if exists {
		if size > 0 && size != doc.SizeBytes {
			doc.SizeBytes = size
		}
		if err := w.repo.UpdateStatus(ctx, doc.ID, StatusUploaded, &doc, nil); err != nil {
			return fmt.Errorf("mark document %s as uploaded: %w", doc.ID, err)
		}
		log.Printf("[file worker] document %s marked as uploaded (key: %s)", doc.ID, doc.ObjectKey)
	} else {
		if err := w.repo.UpdateStatus(ctx, doc.ID, StatusFailed, &doc, nil); err != nil {
			return fmt.Errorf("mark document %s as failed: %w", doc.ID, err)
		}
		log.Printf("[file worker] document %s marked as failed (object not found in R2)", doc.ID)
	}

	return nil
}

func (w *UploadWorker) objectExists(ctx context.Context, objectKey string, presignedProvider upload.PresignedURLProvider, hasPresigned bool) (exists bool, size int64, err error) {
	if hasPresigned {
		size, err = presignedProvider.HeadObject(ctx, objectKey)
		if err != nil {
			if isNotFoundError(err) {
				return false, 0, nil
			}
			return false, 0, err
		}
		return true, size, nil
	}

	rc, err := w.storage.Download(ctx, objectKey)
	if err != nil {
		if isNotFoundError(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	rc.Close()
	return true, 0, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "NoSuchKey") ||
		contains(msg, "404") ||
		contains(msg, "not found") ||
		contains(msg, "NotFound")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

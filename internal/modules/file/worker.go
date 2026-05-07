package file

import (
	"context"
	"fmt"
	"log"
	"strings"
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

	sem := make(chan struct{}, 10)
	for _, doc := range docs {
		doc := doc
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			if err := w.verifyAndUpdate(ctx, doc, presignedProvider, hasPresigned); err != nil {
				log.Printf("[file worker] error processing document %s: %v", doc.ID, err)
			}
		}()
	}
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
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

	log.Printf("[file worker] processing %d expired upload(s)", len(docs))

	presignedProvider, hasPresigned := w.storage.(upload.PresignedURLProvider)

	for _, doc := range docs {
		doc := doc
		exists, size, err := w.objectExists(ctx, doc.ObjectKey, presignedProvider, hasPresigned)
		if err != nil {
			log.Printf("[file worker] error checking expired document %s: %v", doc.ID, err)
			continue
		}

		if exists {
			if size > 0 && size != doc.SizeBytes {
				doc.SizeBytes = size
			}
			if err := w.repo.UpdateStatus(ctx, doc.ID, StatusUploaded, &doc, nil); err != nil {
				log.Printf("[file worker] failed to mark expired document %s as uploaded: %v", doc.ID, err)
				continue
			}
			log.Printf("[file worker] expired document %s was actually uploaded; marked as uploaded", doc.ID)
		} else {
			if err := w.repo.UpdateStatus(ctx, doc.ID, StatusFailed, &doc, nil); err != nil {
				log.Printf("[file worker] failed to mark document %s as failed: %v", doc.ID, err)
				continue
			}
			log.Printf("[file worker] document %s marked as failed (upload expired at %v)", doc.ID, doc.UploadExpiresAt)
		}
	}
}

func (w *UploadWorker) verifyAndUpdate(ctx context.Context, doc Document, presignedProvider upload.PresignedURLProvider, hasPresigned bool) error {
	exists, size, err := w.objectExists(ctx, doc.ObjectKey, presignedProvider, hasPresigned)
	if err != nil {
		return fmt.Errorf("check object existence for %s: %w", doc.ObjectKey, err)
	}

	docCopy := doc
	if exists {
		if size > 0 && size != docCopy.SizeBytes {
			docCopy.SizeBytes = size
		}
		if err := w.repo.UpdateStatus(ctx, docCopy.ID, StatusUploaded, &docCopy, nil); err != nil {
			return fmt.Errorf("mark document %s as uploaded: %w", docCopy.ID, err)
		}
		log.Printf("[file worker] document %s marked as uploaded (key: %s)", docCopy.ID, docCopy.ObjectKey)
	} else {
		// Only fail if the presigned URL has expired — still within window means client hasn't uploaded yet
		if docCopy.UploadExpiresAt != nil && time.Now().Before(*docCopy.UploadExpiresAt) {
			log.Printf("[file worker] document %s not yet uploaded, presign expires at %v — skipping", docCopy.ID, docCopy.UploadExpiresAt)
			return nil
		}
		if err := w.repo.UpdateStatus(ctx, docCopy.ID, StatusFailed, &docCopy, nil); err != nil {
			return fmt.Errorf("mark document %s as failed: %w", docCopy.ID, err)
		}
		log.Printf("[file worker] document %s marked as failed (object not found in storage)", docCopy.ID)
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
	return strings.Contains(msg, "NoSuchKey") ||
		strings.Contains(msg, "404 Not Found") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "NotFound")
}

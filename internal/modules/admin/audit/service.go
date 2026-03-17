package audit

import (
	"context"
	"log"
)

type Service interface {
	Log(ctx context.Context, entry *LogEntry) error
	LogAsync(entry *LogEntry)
	Query(ctx context.Context, params QueryParams) ([]*AuditLog, error)
	GetByID(ctx context.Context, id string) (*AuditLog, error)
}

type service struct {
	repo    Repository
	logChan chan *LogEntry
}

func NewService(repo Repository) Service {
	s := &service{
		repo:    repo,
		logChan: make(chan *LogEntry, 1000), // Buffer for async logging
	}

	// Start async worker
	go s.asyncWorker()

	return s
}

// Log synchronously writes an audit log entry
func (s *service) Log(ctx context.Context, entry *LogEntry) error {
	return s.repo.Insert(ctx, entry)
}

// LogAsync queues an audit log entry for async processing
// This prevents audit logging from blocking main operations
func (s *service) LogAsync(entry *LogEntry) {
	select {
	case s.logChan <- entry:
		// Successfully queued
	default:
		// Channel full, log error but don't block
		log.Printf("WARN: audit log channel full, dropping entry: %s.%s", entry.Module, entry.Action)
	}
}

// asyncWorker processes audit log entries from the queue
func (s *service) asyncWorker() {
	for entry := range s.logChan {
		ctx := context.Background()
		if err := s.repo.Insert(ctx, entry); err != nil {
			// Log error but continue processing
			log.Printf("ERROR: failed to insert audit log: %v (action: %s.%s)", err, entry.Module, entry.Action)
		}
	}
}

// Query retrieves audit logs based on filter parameters
func (s *service) Query(ctx context.Context, params QueryParams) ([]*AuditLog, error) {
	return s.repo.List(ctx, params)
}

// GetByID retrieves a specific audit log entry
func (s *service) GetByID(ctx context.Context, id string) (*AuditLog, error) {
	return s.repo.GetByID(ctx, id)
}

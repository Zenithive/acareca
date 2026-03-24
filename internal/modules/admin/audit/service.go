package audit

import (
	"context"
	"log"

	"github.com/iamarpitzala/acareca/internal/shared/util"
)

type Service interface {
	Log(ctx context.Context, entry *LogEntry) error
	LogAsync(entry *LogEntry)
	Query(ctx context.Context, f *Filter) (*util.RsList, error)
	GetByID(ctx context.Context, id string) (*RsAuditLog, error)
	Shutdown()
}

type service struct {
	repo    Repository
	logChan chan *LogEntry
	done    chan struct{}
}

func NewService(repo Repository) Service {
	s := &service{
		repo:    repo,
		logChan: make(chan *LogEntry, 1000),
		done:    make(chan struct{}),
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
	close(s.done)
}

// Shutdown drains the log channel and waits for the worker to finish.
func (s *service) Shutdown() {
	close(s.logChan)
	<-s.done
}

// Query retrieves audit logs based on filter parameters
func (s *service) Query(ctx context.Context, f *Filter) (*util.RsList, error) {
	ft := f.MapToFilter()

	// Fetch data
	list, err := s.repo.List(ctx, ft)
	if err != nil {
		return nil, err
	}

	// Fetch total count for pagination
	total, err := s.repo.Count(ctx, ft)
	if err != nil {
		return nil, err
	}

	data := make([]*RsAuditLog, 0, len(list))
	for _, item := range list {
		data = append(data, item.ToRs())
	}

	var rsList util.RsList
	rsList.MapToList(data, total, ft.Offset, ft.Limit)

	return &rsList, nil
}

// GetByID retrieves a specific audit log entry
func (s *service) GetByID(ctx context.Context, id string) (*RsAuditLog, error) {
	l, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toRsAuditLog(l), nil
}

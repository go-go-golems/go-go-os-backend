package gepa

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type RunStatus string

const (
	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCanceled  RunStatus = "canceled"
)

var ErrConcurrencyLimitExceeded = errors.New("concurrent run limit reached")

type StartRunRequest struct {
	ScriptID  string         `json:"script_id"`
	Arguments []string       `json:"arguments,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
}

type RunRecord struct {
	RunID       string         `json:"run_id"`
	ScriptID    string         `json:"script_id"`
	Status      RunStatus      `json:"status"`
	Arguments   []string       `json:"arguments,omitempty"`
	Input       map[string]any `json:"input,omitempty"`
	Output      map[string]any `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   time.Time      `json:"started_at,omitempty"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type RunService interface {
	Start(ctx context.Context, script ScriptDescriptor, request StartRunRequest) (RunRecord, error)
	Get(ctx context.Context, runID string) (RunRecord, bool, error)
	Cancel(ctx context.Context, runID string) (RunRecord, bool, error)
}

type InMemoryRunService struct {
	mu              sync.RWMutex
	runs            map[string]*RunRecord
	cancelFuncs     map[string]context.CancelFunc
	completionDelay time.Duration
	runTimeout      time.Duration
	maxConcurrent   int
	now             func() time.Time
}

func NewInMemoryRunService(completionDelay, runTimeout time.Duration, maxConcurrent int) *InMemoryRunService {
	delay := completionDelay
	if delay <= 0 {
		delay = 300 * time.Millisecond
	}
	limit := maxConcurrent
	if limit <= 0 {
		limit = 4
	}
	timeout := runTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &InMemoryRunService{
		runs:            map[string]*RunRecord{},
		cancelFuncs:     map[string]context.CancelFunc{},
		completionDelay: delay,
		runTimeout:      timeout,
		maxConcurrent:   limit,
		now:             time.Now,
	}
}

func (s *InMemoryRunService) Start(ctx context.Context, script ScriptDescriptor, request StartRunRequest) (RunRecord, error) {
	if s == nil {
		return RunRecord{}, fmt.Errorf("run service is nil")
	}
	if stringsTrimmed(request.ScriptID) == "" {
		return RunRecord{}, fmt.Errorf("script_id is required")
	}

	now := s.now()
	runID := "run-" + uuid.NewString()

	s.mu.Lock()
	if s.maxConcurrent > 0 && s.runningCountLocked() >= s.maxConcurrent {
		s.mu.Unlock()
		return RunRecord{}, fmt.Errorf("%w (limit=%d)", ErrConcurrencyLimitExceeded, s.maxConcurrent)
	}
	runCtx, cancel := context.WithTimeout(ctx, s.runTimeout)

	record := &RunRecord{
		RunID:     runID,
		ScriptID:  script.ID,
		Status:    RunStatusRunning,
		Arguments: append([]string(nil), request.Arguments...),
		Input:     cloneMap(request.Input),
		Output: map[string]any{
			"script_path": script.Path,
		},
		CreatedAt: now,
		StartedAt: now,
		UpdatedAt: now,
	}

	s.runs[runID] = record
	s.cancelFuncs[runID] = cancel
	s.mu.Unlock()

	go s.completeRunAfterDelay(runCtx, runID, script)
	return cloneRunRecord(record), nil
}

func (s *InMemoryRunService) completeRunAfterDelay(ctx context.Context, runID string, script ScriptDescriptor) {
	timer := time.NewTimer(s.completionDelay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		s.mu.Lock()
		record, ok := s.runs[runID]
		if ok && record.Status == RunStatusRunning && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			now := s.now()
			record.Status = RunStatusFailed
			record.Error = "run timed out"
			record.UpdatedAt = now
			record.CompletedAt = now
		}
		delete(s.cancelFuncs, runID)
		s.mu.Unlock()
		return
	case <-timer.C:
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.runs[runID]
	if !ok || record.Status != RunStatusRunning {
		return
	}
	now := s.now()
	record.Status = RunStatusCompleted
	record.UpdatedAt = now
	record.CompletedAt = now
	record.Output = map[string]any{
		"message":     "run completed in internal placeholder runtime",
		"script_id":   script.ID,
		"script_name": script.Name,
	}
	delete(s.cancelFuncs, runID)
}

func (s *InMemoryRunService) Get(_ context.Context, stringRunID string) (RunRecord, bool, error) {
	if s == nil {
		return RunRecord{}, false, fmt.Errorf("run service is nil")
	}
	runID := stringsTrimmed(stringRunID)
	if runID == "" {
		return RunRecord{}, false, nil
	}

	s.mu.RLock()
	record, ok := s.runs[runID]
	s.mu.RUnlock()
	if !ok {
		return RunRecord{}, false, nil
	}
	return cloneRunRecord(record), true, nil
}

func (s *InMemoryRunService) Cancel(_ context.Context, stringRunID string) (RunRecord, bool, error) {
	if s == nil {
		return RunRecord{}, false, fmt.Errorf("run service is nil")
	}
	runID := stringsTrimmed(stringRunID)
	if runID == "" {
		return RunRecord{}, false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.runs[runID]
	if !ok {
		return RunRecord{}, false, nil
	}
	if record.Status == RunStatusCompleted || record.Status == RunStatusFailed || record.Status == RunStatusCanceled {
		return cloneRunRecord(record), true, nil
	}

	now := s.now()
	record.Status = RunStatusCanceled
	record.UpdatedAt = now
	record.CompletedAt = now
	if cancel, ok := s.cancelFuncs[runID]; ok {
		cancel()
		delete(s.cancelFuncs, runID)
	}

	return cloneRunRecord(record), true, nil
}

func cloneRunRecord(in *RunRecord) RunRecord {
	if in == nil {
		return RunRecord{}
	}
	out := *in
	out.Arguments = append([]string(nil), in.Arguments...)
	out.Input = cloneMap(in.Input)
	out.Output = cloneMap(in.Output)
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringsTrimmed(s string) string {
	return strings.TrimSpace(s)
}

func (s *InMemoryRunService) runningCountLocked() int {
	running := 0
	for _, run := range s.runs {
		if run != nil && run.Status == RunStatusRunning {
			running++
		}
	}
	return running
}

package gepa

import "context"

// GepaRuntime is the module-internal runtime contract used by handlers.
// This isolates transport/HTTP handlers from concrete run/catalog implementations.
type GepaRuntime interface {
	ListScripts(ctx context.Context) ([]ScriptDescriptor, error)
	StartRun(ctx context.Context, request StartRunRequest) (RunRecord, error)
	GetRun(ctx context.Context, runID string) (RunRecord, bool, error)
	CancelRun(ctx context.Context, runID string) (RunRecord, bool, error)
	ListEvents(ctx context.Context, runID string, afterSeq int64) ([]RunEvent, bool, error)
}

type InMemoryRuntime struct {
	catalog ScriptCatalog
	runs    RunService
}

func NewInMemoryRuntime(catalog ScriptCatalog, runs RunService) *InMemoryRuntime {
	return &InMemoryRuntime{
		catalog: catalog,
		runs:    runs,
	}
}

func (r *InMemoryRuntime) ListScripts(ctx context.Context) ([]ScriptDescriptor, error) {
	return r.catalog.List(ctx)
}

func (r *InMemoryRuntime) StartRun(ctx context.Context, request StartRunRequest) (RunRecord, error) {
	if request.ScriptID == "" {
		return RunRecord{}, ErrScriptIDRequired
	}
	script, found, err := r.findScript(ctx, request.ScriptID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrUnknownScriptID
	}
	return r.runs.Start(ctx, script, request)
}

func (r *InMemoryRuntime) GetRun(ctx context.Context, runID string) (RunRecord, bool, error) {
	return r.runs.Get(ctx, runID)
}

func (r *InMemoryRuntime) CancelRun(ctx context.Context, runID string) (RunRecord, bool, error) {
	return r.runs.Cancel(ctx, runID)
}

func (r *InMemoryRuntime) ListEvents(ctx context.Context, runID string, afterSeq int64) ([]RunEvent, bool, error) {
	return r.runs.Events(ctx, runID, afterSeq)
}

func (r *InMemoryRuntime) findScript(ctx context.Context, scriptID string) (ScriptDescriptor, bool, error) {
	scripts, err := r.catalog.List(ctx)
	if err != nil {
		return ScriptDescriptor{}, false, err
	}
	for _, script := range scripts {
		if script.ID == scriptID {
			return script, true, nil
		}
	}
	return ScriptDescriptor{}, false, nil
}

package gepa

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModule_ListScriptsAndStartRun(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "hello.js"), []byte("console.log('hi')"), 0o600))

	module, err := NewModule(ModuleConfig{
		ScriptsRoots:       []string{tmp},
		EnableReflection:   true,
		RunCompletionDelay: 2 * time.Second,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	require.NoError(t, module.MountRoutes(mux))

	scriptsReq := httptest.NewRequest(http.MethodGet, "/scripts", nil)
	scriptsRR := httptest.NewRecorder()
	mux.ServeHTTP(scriptsRR, scriptsReq)
	require.Equal(t, http.StatusOK, scriptsRR.Code)

	var scriptsPayload struct {
		Scripts []ScriptDescriptor `json:"scripts"`
	}
	require.NoError(t, json.NewDecoder(scriptsRR.Body).Decode(&scriptsPayload))
	require.Len(t, scriptsPayload.Scripts, 1)
	require.Equal(t, "hello.js", scriptsPayload.Scripts[0].ID)

	startBody := []byte(`{"script_id":"hello.js","arguments":["--once"]}`)
	startReq := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader(startBody))
	startRR := httptest.NewRecorder()
	mux.ServeHTTP(startRR, startReq)
	require.Equal(t, http.StatusCreated, startRR.Code)

	var startPayload struct {
		Run RunRecord `json:"run"`
	}
	require.NoError(t, json.NewDecoder(startRR.Body).Decode(&startPayload))
	require.Equal(t, "hello.js", startPayload.Run.ScriptID)
	require.Equal(t, RunStatusRunning, startPayload.Run.Status)

	getReq := httptest.NewRequest(http.MethodGet, "/runs/"+startPayload.Run.RunID, nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	require.Equal(t, http.StatusOK, getRR.Code)

	cancelReq := httptest.NewRequest(http.MethodPost, "/runs/"+startPayload.Run.RunID+"/cancel", nil)
	cancelRR := httptest.NewRecorder()
	mux.ServeHTTP(cancelRR, cancelReq)
	require.Equal(t, http.StatusOK, cancelRR.Code)

	var cancelPayload struct {
		Run RunRecord `json:"run"`
	}
	require.NoError(t, json.NewDecoder(cancelRR.Body).Decode(&cancelPayload))
	require.Equal(t, RunStatusCanceled, cancelPayload.Run.Status)
}

func TestModule_ReflectionAndSchemas(t *testing.T) {
	module, err := NewModule(ModuleConfig{
		EnableReflection: true,
	})
	require.NoError(t, err)

	doc, err := module.Reflection(context.Background())
	require.NoError(t, err)
	require.Equal(t, AppID, doc.AppID)
	require.NotEmpty(t, doc.APIs)
	require.NotEmpty(t, doc.Schemas)

	mux := http.NewServeMux()
	require.NoError(t, module.MountRoutes(mux))

	schemaReq := httptest.NewRequest(http.MethodGet, "/schemas/gepa.runs.start.request.v1", nil)
	schemaRR := httptest.NewRecorder()
	mux.ServeHTTP(schemaRR, schemaReq)
	require.Equal(t, http.StatusOK, schemaRR.Code)

	unknownReq := httptest.NewRequest(http.MethodGet, "/schemas/does.not.exist", nil)
	unknownRR := httptest.NewRecorder()
	mux.ServeHTTP(unknownRR, unknownReq)
	require.Equal(t, http.StatusNotFound, unknownRR.Code)
}

func TestModule_HealthFailsForMissingRoot(t *testing.T) {
	module, err := NewModule(ModuleConfig{
		EnableReflection: true,
		ScriptsRoots:     []string{"/path/that/does/not/exist"},
	})
	require.NoError(t, err)
	require.Error(t, module.Health(context.Background()))
}

func TestModule_RunTimeoutMarksRunFailed(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "timeout.js"), []byte("console.log('timeout')"), 0o600))

	module, err := NewModule(ModuleConfig{
		ScriptsRoots:       []string{tmp},
		EnableReflection:   true,
		RunCompletionDelay: 2 * time.Second,
		RunTimeout:         100 * time.Millisecond,
		MaxConcurrentRuns:  2,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	require.NoError(t, module.MountRoutes(mux))

	startReq := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader([]byte(`{"script_id":"timeout.js"}`)))
	startRR := httptest.NewRecorder()
	mux.ServeHTTP(startRR, startReq)
	require.Equal(t, http.StatusCreated, startRR.Code)

	var startPayload struct {
		Run RunRecord `json:"run"`
	}
	require.NoError(t, json.NewDecoder(startRR.Body).Decode(&startPayload))

	time.Sleep(250 * time.Millisecond)

	getReq := httptest.NewRequest(http.MethodGet, "/runs/"+startPayload.Run.RunID, nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	require.Equal(t, http.StatusOK, getRR.Code)

	var getPayload struct {
		Run RunRecord `json:"run"`
	}
	require.NoError(t, json.NewDecoder(getRR.Body).Decode(&getPayload))
	require.Equal(t, RunStatusFailed, getPayload.Run.Status)
	require.Equal(t, "run timed out", getPayload.Run.Error)
}

func TestModule_EnforcesMaxConcurrentRuns(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "one.js"), []byte("console.log('one')"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "two.js"), []byte("console.log('two')"), 0o600))

	module, err := NewModule(ModuleConfig{
		ScriptsRoots:       []string{tmp},
		EnableReflection:   true,
		RunCompletionDelay: 2 * time.Second,
		RunTimeout:         30 * time.Second,
		MaxConcurrentRuns:  1,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	require.NoError(t, module.MountRoutes(mux))

	firstReq := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader([]byte(`{"script_id":"one.js"}`)))
	firstRR := httptest.NewRecorder()
	mux.ServeHTTP(firstRR, firstReq)
	require.Equal(t, http.StatusCreated, firstRR.Code)

	secondReq := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader([]byte(`{"script_id":"two.js"}`)))
	secondRR := httptest.NewRecorder()
	mux.ServeHTTP(secondRR, secondReq)
	require.Equal(t, http.StatusTooManyRequests, secondRR.Code)
}

func TestModule_EventsAndTimelineEndpoints(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "events.js"), []byte("console.log('events')"), 0o600))

	module, err := NewModule(ModuleConfig{
		ScriptsRoots:       []string{tmp},
		EnableReflection:   true,
		RunCompletionDelay: 150 * time.Millisecond,
		RunTimeout:         5 * time.Second,
		MaxConcurrentRuns:  2,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	require.NoError(t, module.MountRoutes(mux))

	startReq := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader([]byte(`{"script_id":"events.js"}`)))
	startRR := httptest.NewRecorder()
	mux.ServeHTTP(startRR, startReq)
	require.Equal(t, http.StatusCreated, startRR.Code)

	var startPayload struct {
		Run RunRecord `json:"run"`
	}
	require.NoError(t, json.NewDecoder(startRR.Body).Decode(&startPayload))
	runID := startPayload.Run.RunID

	eventsReq := httptest.NewRequest(http.MethodGet, "/runs/"+runID+"/events?afterSeq=0", nil)
	eventsRR := httptest.NewRecorder()
	mux.ServeHTTP(eventsRR, eventsReq)
	require.Equal(t, http.StatusOK, eventsRR.Code)
	body := eventsRR.Body.String()
	require.Contains(t, body, "event: run.started")
	require.Contains(t, body, "event: run.completed")

	timelineReq := httptest.NewRequest(http.MethodGet, "/runs/"+runID+"/timeline", nil)
	timelineRR := httptest.NewRecorder()
	mux.ServeHTTP(timelineRR, timelineReq)
	require.Equal(t, http.StatusOK, timelineRR.Code)

	var timelinePayload map[string]any
	require.NoError(t, json.NewDecoder(timelineRR.Body).Decode(&timelinePayload))
	require.Equal(t, runID, timelinePayload["run_id"])
	require.Equal(t, string(RunStatusCompleted), timelinePayload["status"])
	counts, ok := timelinePayload["counts"].(map[string]any)
	require.True(t, ok)
	_, hasStarted := counts["run.started"]
	require.True(t, hasStarted)
	_, hasCompleted := counts["run.completed"]
	require.True(t, hasCompleted)
}

func TestModule_EventsEndpointRejectsInvalidAfterSeq(t *testing.T) {
	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "events.js"), []byte("console.log('events')"), 0o600))

	module, err := NewModule(ModuleConfig{
		ScriptsRoots:       []string{tmp},
		EnableReflection:   true,
		RunCompletionDelay: 150 * time.Millisecond,
	})
	require.NoError(t, err)

	mux := http.NewServeMux()
	require.NoError(t, module.MountRoutes(mux))

	startReq := httptest.NewRequest(http.MethodPost, "/runs", bytes.NewReader([]byte(`{"script_id":"events.js"}`)))
	startRR := httptest.NewRecorder()
	mux.ServeHTTP(startRR, startReq)
	require.Equal(t, http.StatusCreated, startRR.Code)

	var startPayload struct {
		Run RunRecord `json:"run"`
	}
	require.NoError(t, json.NewDecoder(startRR.Body).Decode(&startPayload))

	req := httptest.NewRequest(http.MethodGet, "/runs/"+startPayload.Run.RunID+"/events?afterSeq=abc", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	payload, err := io.ReadAll(rr.Body)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(payload), "invalid afterSeq"))
}

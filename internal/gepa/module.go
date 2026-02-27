package gepa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/backendhost"
)

const (
	AppID = "gepa"
)

type ModuleConfig struct {
	ScriptsRoots       []string
	EnableReflection   bool
	RunCompletionDelay time.Duration
	RunTimeout         time.Duration
	MaxConcurrentRuns  int
}

type Module struct {
	config  ModuleConfig
	catalog ScriptCatalog
	runs    RunService
}

func NewModule(config ModuleConfig) (*Module, error) {
	catalog := NewFileScriptCatalog(config.ScriptsRoots)
	runs := NewInMemoryRunService(config.RunCompletionDelay, config.RunTimeout, config.MaxConcurrentRuns)
	return NewModuleWithDeps(config, catalog, runs)
}

func NewModuleWithDeps(config ModuleConfig, catalog ScriptCatalog, runs RunService) (*Module, error) {
	if catalog == nil {
		return nil, fmt.Errorf("gepa script catalog is nil")
	}
	if runs == nil {
		return nil, fmt.Errorf("gepa run service is nil")
	}
	if len(config.ScriptsRoots) > 0 {
		roots := make([]string, 0, len(config.ScriptsRoots))
		for _, root := range config.ScriptsRoots {
			trimmed := strings.TrimSpace(root)
			if trimmed == "" {
				continue
			}
			roots = append(roots, trimmed)
		}
		slices.Sort(roots)
		config.ScriptsRoots = roots
	}
	return &Module{
		config:  config,
		catalog: catalog,
		runs:    runs,
	}, nil
}

func (m *Module) Manifest() backendhost.AppBackendManifest {
	return backendhost.AppBackendManifest{
		AppID:       AppID,
		Name:        "GEPA",
		Description: "GEPA script runner backend module",
		Required:    false,
		Capabilities: []string{
			"script-runner",
			"schemas",
			"reflection",
		},
	}
}

func (m *Module) MountRoutes(mux *http.ServeMux) error {
	if mux == nil {
		return fmt.Errorf("gepa module mount mux is nil")
	}
	mux.HandleFunc("/scripts", m.handleListScripts)
	mux.HandleFunc("/scripts/", m.handleListScripts)
	mux.HandleFunc("/runs", m.handleStartRun)
	mux.HandleFunc("/runs/", m.handleRunsSubresource)
	mux.HandleFunc("/schemas/", m.handleSchemaByID)
	return nil
}

func (m *Module) Init(context.Context) error {
	if m == nil {
		return fmt.Errorf("gepa module is nil")
	}
	return nil
}

func (m *Module) Start(context.Context) error {
	if m == nil {
		return fmt.Errorf("gepa module is nil")
	}
	return nil
}

func (m *Module) Stop(context.Context) error {
	return nil
}

func (m *Module) Health(context.Context) error {
	if m == nil {
		return fmt.Errorf("gepa module is nil")
	}
	for _, root := range m.config.ScriptsRoots {
		info, err := os.Stat(root)
		if err != nil {
			return fmt.Errorf("scripts root %q is not available: %w", root, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("scripts root %q is not a directory", root)
		}
	}
	return nil
}

func (m *Module) Reflection(context.Context) (*backendhost.ModuleReflectionDocument, error) {
	if !m.config.EnableReflection {
		return nil, fmt.Errorf("reflection is disabled for module %q", AppID)
	}
	basePath := "/api/apps/" + AppID
	return &backendhost.ModuleReflectionDocument{
		AppID:   AppID,
		Name:    "GEPA",
		Version: "v1",
		Summary: "In-process GEPA script module with discoverable API and schemas",
		Capabilities: []backendhost.ReflectionCapability{
			{ID: "script-runner", Stability: "beta", Description: "List and run local GEPA scripts"},
			{ID: "schemas", Stability: "stable", Description: "Expose request/response schemas by id"},
			{ID: "reflection", Stability: "stable", Description: "Expose module API/docs/schema metadata"},
		},
		Docs: []backendhost.ReflectionDocLink{
			{
				ID:          "part-1-backendmodule-design",
				Title:       "Part 1: Internal BackendModule integration only",
				Path:        "go-go-gepa/ttmp/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP--backend-roadmap-for-gepa-in-process-integration-and-external-plugin-extraction/design-doc/03-part-1-internal-backendmodule-integration-only.md",
				Description: "Design document for in-process module contracts",
			},
		},
		APIs: []backendhost.ReflectionAPI{
			{
				ID:             "list-scripts",
				Method:         http.MethodGet,
				Path:           basePath + "/scripts",
				Summary:        "List local GEPA scripts",
				ResponseSchema: "gepa.scripts.list.response.v1",
				ErrorSchema:    "gepa.error.v1",
			},
			{
				ID:             "start-run",
				Method:         http.MethodPost,
				Path:           basePath + "/runs",
				Summary:        "Start a GEPA run using a known script id",
				RequestSchema:  "gepa.runs.start.request.v1",
				ResponseSchema: "gepa.runs.start.response.v1",
				ErrorSchema:    "gepa.error.v1",
			},
			{
				ID:             "get-run",
				Method:         http.MethodGet,
				Path:           basePath + "/runs/{run_id}",
				Summary:        "Get status for one GEPA run",
				ResponseSchema: "gepa.runs.get.response.v1",
				ErrorSchema:    "gepa.error.v1",
			},
			{
				ID:             "cancel-run",
				Method:         http.MethodPost,
				Path:           basePath + "/runs/{run_id}/cancel",
				Summary:        "Cancel a running GEPA run",
				ResponseSchema: "gepa.runs.get.response.v1",
				ErrorSchema:    "gepa.error.v1",
			},
		},
		Schemas: []backendhost.ReflectionSchemaRef{
			{ID: "gepa.scripts.list.response.v1", Format: "json-schema", URI: basePath + "/schemas/gepa.scripts.list.response.v1"},
			{ID: "gepa.runs.start.request.v1", Format: "json-schema", URI: basePath + "/schemas/gepa.runs.start.request.v1"},
			{ID: "gepa.runs.start.response.v1", Format: "json-schema", URI: basePath + "/schemas/gepa.runs.start.response.v1"},
			{ID: "gepa.runs.get.response.v1", Format: "json-schema", URI: basePath + "/schemas/gepa.runs.get.response.v1"},
			{ID: "gepa.error.v1", Format: "json-schema", URI: basePath + "/schemas/gepa.error.v1"},
		},
	}, nil
}

func (m *Module) handleListScripts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	scripts, err := m.catalog.List(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scripts": scripts,
	})
}

func (m *Module) handleStartRun(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var payload StartRunRequest
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	payload.ScriptID = strings.TrimSpace(payload.ScriptID)
	if payload.ScriptID == "" {
		writeJSONError(w, http.StatusBadRequest, "script_id is required")
		return
	}

	script, found, err := m.findScript(req.Context(), payload.ScriptID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeJSONError(w, http.StatusBadRequest, "unknown script_id")
		return
	}

	run, err := m.runs.Start(req.Context(), script, payload)
	if err != nil {
		if errors.Is(err, ErrConcurrencyLimitExceeded) {
			writeJSONError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"run": run,
	})
}

func (m *Module) handleRunsSubresource(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/runs/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, req)
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) == 1 {
		if req.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		runID := strings.TrimSpace(parts[0])
		run, ok, err := m.runs.Get(req.Context(), runID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, req)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"run": run})
		return
	}

	if len(parts) == 2 && parts[1] == "cancel" {
		if req.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		runID := strings.TrimSpace(parts[0])
		run, ok, err := m.runs.Cancel(req.Context(), runID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			http.NotFound(w, req)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"run": run})
		return
	}

	http.NotFound(w, req)
}

func (m *Module) handleSchemaByID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	schemaID := strings.TrimPrefix(req.URL.Path, "/schemas/")
	schemaID = strings.TrimSpace(strings.Trim(schemaID, "/"))
	if schemaID == "" {
		http.NotFound(w, req)
		return
	}
	doc, ok := schemaDocuments[schemaID]
	if !ok {
		http.NotFound(w, req)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (m *Module) findScript(ctx context.Context, scriptID string) (ScriptDescriptor, bool, error) {
	scripts, err := m.catalog.List(ctx)
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

func writeMethodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": strings.TrimSpace(msg),
	})
}

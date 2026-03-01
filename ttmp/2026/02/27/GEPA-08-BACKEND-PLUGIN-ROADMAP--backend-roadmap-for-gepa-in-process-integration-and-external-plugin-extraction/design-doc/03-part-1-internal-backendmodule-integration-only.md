---
Title: 'Part 1: Internal BackendModule integration only'
Ticket: GEPA-08-BACKEND-PLUGIN-ROADMAP
Status: active
Topics:
    - gepa
    - plugins
    - backend
    - architecture
    - go-go-os
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/module.go
      Note: Base AppBackendModule contract and lifecycle hooks.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/routes.go
      Note: Namespaced route mounting and alias prevention.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/manifest_endpoint.go
      Note: /api/os/apps manifest response currently exposed.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go
      Note: Module registry wiring entrypoint.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/inventory_backend_module.go
      Note: Existing internal module pattern to mirror.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/pinoweb/middleware_definitions.go
      Note: Existing schema source shape via ConfigJSONSchema.
ExternalSources: []
Summary: Part-1-only design for internal BackendModule integration and host-side reflection. Excludes generic external runtime protocol and runtime event transport design.
LastUpdated: 2026-02-27T12:58:00-05:00
WhatFor: Provide implementation-ready guidance for building the in-process BackendModule and reflection surfaces first.
WhenToUse: Use this as the source of truth for Phase 1 implementation before any plugin-process extraction work.
---

# Part 1: Internal BackendModule integration only

## Executive Summary

This document defines only Phase 1: an in-process module integration that conforms to the existing `AppBackendModule` host model in `go-go-os`.

It intentionally does not include:

- generic external runtime protocol design,
- cross-process event transport contracts,
- plugin process manager semantics.

The goal is to finish a clean, production-usable internal module first, with additive reflection endpoints that help the frontend and tooling discover module APIs, schemas, and docs.

## Problem Statement

The backend host already supports module lifecycle and namespaced routes, but developers still rely on source diving to answer operational questions:

- Which routes does a module expose?
- Which capabilities are stable versus experimental?
- Where are payload schemas and docs?
- Which module version/capability set is deployed?

For Phase 1, we can solve this without building a new plugin runtime abstraction yet.

The missing piece is not execution. The missing piece is discoverability and explicit contracts around internal module APIs.

## Scope

### In scope

- Internal module implementation (`AppBackendModule`) for GEPA.
- Route namespace under `/api/apps/gepa/*` only.
- Additive reflection endpoints for API/documentation/schema discovery.
- Host manifest enrichment via additive fields.
- Test plan for lifecycle, route invariants, and reflection contract stability.

### Out of scope

- Generic module process protocol.
- Runtime event streaming protocol design for external processes.
- Event schema federation and transport negotiation.
- Frontend federation/runtime loading strategy.

## Current Baseline (What We Reuse)

The current host already provides most of the mechanics we need.

### Baseline contract

`internal/backendhost/module.go` defines the key contract:

```go
type AppBackendModule interface {
    Manifest() AppManifest
    RegisterRoutes(r chi.Router) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) error
}
```

This is sufficient for Phase 1. We should not replace it.

### Route behavior

`internal/backendhost/routes.go` already enforces namespaced mounting with no legacy alias fallback. This matches the intended architecture and avoids endpoint drift.

### Existing module pattern

`cmd/go-go-os-launcher/inventory_backend_module.go` is a practical template for how to wire dependencies, register routes, and expose manifest metadata.

## Proposed Solution (Part 1 Only)

### 1) Keep `AppBackendModule` as the core interface

No breaking changes to existing modules.

Add optional reflection support via a separate interface that modules may implement.

```go
// Optional, additive.
type ReflectiveAppBackendModule interface {
    Reflection(ctx context.Context) (*ModuleReflectionDocument, error)
}
```

Host behavior:

- If module implements `ReflectiveAppBackendModule`, expose reflection endpoints.
- If not, return a minimal fallback reflection payload derived from manifest.

### 2) Build internal GEPA module around existing host contracts

Create internal package layout in `go-go-os/go-inventory-chat`:

```text
internal/gepa/
  module.go
  handlers.go
  reflection.go
  schemas.go
```

Responsibilities:

- `module.go`: lifecycle, manifest, dependency wiring.
- `handlers.go`: namespaced HTTP handlers.
- `reflection.go`: returns reflection metadata.
- `schemas.go`: static or computed JSON schema references for payloads.

### 3) Add reflection endpoints at host level

Keep `/api/os/apps` unchanged for existing clients, but add optional fields.

Add new endpoint:

- `GET /api/os/apps/{app_id}/reflection`

This endpoint returns machine-readable module metadata for API discovery and docs linking.

No runtime event protocol is introduced in this phase.

## Reflection Data Model (Part 1)

The reflection payload is intentionally focused on APIs/docs/schemas. Event transport details are excluded.

```go
type ModuleReflectionDocument struct {
    AppID       string                    `json:"app_id"`
    Name        string                    `json:"name"`
    Version     string                    `json:"version,omitempty"`
    Summary     string                    `json:"summary,omitempty"`
    Docs        []ReflectionDocLink       `json:"docs,omitempty"`
    Capabilities []ReflectionCapability   `json:"capabilities,omitempty"`
    APIs        []ReflectionAPI           `json:"apis,omitempty"`
    Schemas     []ReflectionSchemaRef     `json:"schemas,omitempty"`
}

type ReflectionDocLink struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    URL         string `json:"url,omitempty"`
    Path        string `json:"path,omitempty"`
    Description string `json:"description,omitempty"`
}

type ReflectionCapability struct {
    ID          string `json:"id"`
    Stability   string `json:"stability,omitempty"` // stable|beta|experimental
    Description string `json:"description,omitempty"`
}

type ReflectionAPI struct {
    ID             string                `json:"id"`
    Method         string                `json:"method"`
    Path           string                `json:"path"`
    Summary        string                `json:"summary,omitempty"`
    RequestSchema  string                `json:"request_schema,omitempty"`
    ResponseSchema string                `json:"response_schema,omitempty"`
    ErrorSchema    string                `json:"error_schema,omitempty"`
    Tags           []string              `json:"tags,omitempty"`
}

type ReflectionSchemaRef struct {
    ID       string `json:"id"`
    Format   string `json:"format"` // json-schema|protobuf|openapi
    URI      string `json:"uri,omitempty"`
    Embedded any    `json:"embedded,omitempty"`
}
```

### Constraints

- Additive only: no existing endpoint removals.
- Stable identifiers (`api.id`, `schema.id`) once published.
- Payload must be deterministic for snapshot testing.

## Endpoint Contract (Part 1)

### Existing endpoint (additive extension)

`GET /api/os/apps`

Add optional reflection hints:

```json
{
  "apps": [
    {
      "app_id": "gepa",
      "name": "GEPA",
      "required": false,
      "capabilities": ["script-runner", "timeline"],
      "health": {"status": "healthy"},
      "reflection": {
        "available": true,
        "url": "/api/os/apps/gepa/reflection",
        "version": "v1"
      }
    }
  ]
}
```

### New endpoint

`GET /api/os/apps/{app_id}/reflection`

- `200 OK`: full reflection document.
- `404`: unknown app id.
- `501`: app registered but reflection not implemented (optional fallback behavior; choose one and keep consistent).

Example:

```json
{
  "app_id": "gepa",
  "name": "GEPA",
  "version": "0.1.0",
  "summary": "Local JS optimizer script execution module",
  "docs": [
    {
      "id": "overview",
      "title": "GEPA backend module overview",
      "path": "ttmp/.../design-doc/03-part-1-internal-backendmodule-integration-only.md"
    }
  ],
  "capabilities": [
    {
      "id": "script-runner",
      "stability": "beta",
      "description": "Run local JS scripts with structured metadata"
    }
  ],
  "apis": [
    {
      "id": "list-scripts",
      "method": "GET",
      "path": "/api/apps/gepa/scripts",
      "summary": "List locally discoverable GEPA scripts",
      "response_schema": "gepa.scripts.list.response.v1"
    },
    {
      "id": "start-run",
      "method": "POST",
      "path": "/api/apps/gepa/runs",
      "summary": "Start a GEPA script run",
      "request_schema": "gepa.runs.start.request.v1",
      "response_schema": "gepa.runs.start.response.v1"
    }
  ],
  "schemas": [
    {
      "id": "gepa.runs.start.request.v1",
      "format": "json-schema",
      "uri": "/api/apps/gepa/schemas/gepa.runs.start.request.v1"
    }
  ]
}
```

## GEPA Module Design (Internal Only)

### Constructor and dependencies

```go
type GEPAConfig struct {
    EnableReflection bool
    ScriptsRoot      string
}

type GEPAModule struct {
    cfg        GEPAConfig
    logger     *zap.Logger
    runService RunService
    catalog    ScriptCatalog
}

func NewGEPAModule(cfg GEPAConfig, logger *zap.Logger, runService RunService, catalog ScriptCatalog) (*GEPAModule, error) {
    if logger == nil || runService == nil || catalog == nil {
        return nil, fmt.Errorf("gepa module dependencies must be non-nil")
    }
    return &GEPAModule{cfg: cfg, logger: logger, runService: runService, catalog: catalog}, nil
}
```

### Module contract implementation

```go
func (m *GEPAModule) Manifest() backendhost.AppManifest {
    return backendhost.AppManifest{
        AppID:        "gepa",
        Name:         "GEPA",
        Required:     false,
        Capabilities: []string{"script-runner", "timeline"},
    }
}

func (m *GEPAModule) RegisterRoutes(r chi.Router) error {
    r.Get("/scripts", m.handleListScripts)
    r.Post("/runs", m.handleStartRun)
    r.Get("/runs/{run_id}", m.handleGetRun)
    r.Post("/runs/{run_id}/cancel", m.handleCancelRun)
    r.Get("/schemas/{schema_id}", m.handleSchema)
    return nil
}

func (m *GEPAModule) Start(ctx context.Context) error  { return m.runService.Warmup(ctx) }
func (m *GEPAModule) Stop(ctx context.Context) error   { return m.runService.Shutdown(ctx) }
func (m *GEPAModule) Health(ctx context.Context) error { return m.runService.Health(ctx) }
```

### Optional reflection implementation

```go
func (m *GEPAModule) Reflection(ctx context.Context) (*backendhost.ModuleReflectionDocument, error) {
    if !m.cfg.EnableReflection {
        return nil, backendhost.ErrReflectionDisabled
    }
    return buildGEPAReflectionDoc(), nil
}
```

## Host Integration Changes (Minimal)

### Route registration

In module host wiring:

1. Keep `MountNamespacedRoutes` unchanged.
2. Add a small host endpoint handler for `/api/os/apps/{app_id}/reflection`.
3. Lookup module by app id and type-assert optional reflection interface.

Pseudocode:

```go
func (h *Host) HandleModuleReflection(w http.ResponseWriter, r *http.Request) {
    appID := chi.URLParam(r, "app_id")
    mod, ok := h.registry.Get(appID)
    if !ok {
        render404(w, "unknown app")
        return
    }

    reflective, ok := mod.(ReflectiveAppBackendModule)
    if !ok {
        render501(w, "reflection not implemented")
        return
    }

    doc, err := reflective.Reflection(r.Context())
    if err != nil {
        render500(w, err)
        return
    }

    renderJSON(w, http.StatusOK, doc)
}
```

### Manifest enrichment

`/api/os/apps` can include reflection hints without breaking old clients.

```go
type ReflectionHint struct {
    Available bool   `json:"available"`
    URL       string `json:"url,omitempty"`
    Version   string `json:"version,omitempty"`
}
```

## Diagram: Internal-Only Phase

```mermaid
flowchart TD
    A[Backend Host] --> B[Module Registry]
    B --> C[GEPA Module - AppBackendModule]
    A --> D[/api/os/apps]
    A --> E[/api/os/apps/gepa/reflection]
    C --> F[/api/apps/gepa/scripts]
    C --> G[/api/apps/gepa/runs]
    C --> H[/api/apps/gepa/schemas/{id}]
```

## Design Decisions

### Decision 1: additive interface, no base interface break

Rationale:

- avoids migrating all existing modules now,
- keeps risk local to new functionality,
- allows gradual adoption.

### Decision 2: reflection endpoint under `/api/os/apps/{app_id}`

Rationale:

- naturally grouped with module manifest and health,
- easy client discovery from `/api/os/apps`.

### Decision 3: schema references first, embedded schema optional

Rationale:

- keeps initial response payload lean,
- allows existing schema emitters to be reused,
- avoids one large mutable payload format.

## Alternatives Considered

### Alternative A: extend `AppBackendModule` directly with `Reflection()`

Rejected for Part 1 because it forces immediate changes across every module implementation.

### Alternative B: infer reflection automatically by route introspection only

Rejected because route introspection alone cannot produce meaningful docs, schema links, or stable operation IDs.

### Alternative C: postpone reflection until plugin-process phase

Rejected because we need immediate discoverability for internal integration and frontend/API consumer onboarding.

## Implementation Plan (Part 1)

### Step 1: Host-level reflection interface + endpoint

- Add `ReflectiveAppBackendModule` in `internal/backendhost`.
- Add reflection endpoint handler.
- Add manifest reflection hint fields (optional).

### Step 2: GEPA internal module scaffold

- Create `internal/gepa/module.go` and handlers.
- Register in `cmd/go-go-os-launcher/main.go`.
- Ensure health/start/stop semantics follow host lifecycle.

### Step 3: Reflection document assembly

- Add `reflection.go` to GEPA module.
- Define stable IDs for APIs and schemas.
- Expose schema retrieval endpoint(s) for referenced IDs.

### Step 4: Contract tests and snapshot tests

- Test route mount namespace.
- Test `/api/os/apps` includes reflection hints for GEPA.
- Snapshot-test `/api/os/apps/gepa/reflection` determinism.
- Validate referenced schema URLs resolve.

### Step 5: Documentation integration

- Update ticket docs and backend README with route table.
- Add a short runbook for consuming reflection in frontend clients.

## Test Strategy

### Unit tests

- reflection builder returns stable ordering (sort by `id`).
- invalid schema references are rejected at module startup.

### Integration tests

- host returns `404` for unknown app reflection.
- host returns `501` for non-reflective modules (if configured behavior).
- GEPA reflection endpoint returns `200` with expected IDs.

### Regression tests

- existing `/api/os/apps` consumers continue to decode payloads.
- no new non-namespaced aliases introduced.

## Open Questions

1. For non-reflective modules, should response be `404` or `501`? Pick one globally and keep it consistent.
2. Should reflection docs include file paths in production builds, or only stable logical doc IDs?
3. Should schema endpoints be per-module (`/api/apps/{id}/schemas/*`) only, or also mirrored globally under `/api/os/schemas/*`?

## References

- `go-go-os/go-inventory-chat/internal/backendhost/module.go`
- `go-go-os/go-inventory-chat/internal/backendhost/routes.go`
- `go-go-os/go-inventory-chat/internal/backendhost/manifest_endpoint.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/inventory_backend_module.go`
- `go-go-os/go-inventory-chat/internal/pinoweb/middleware_definitions.go`

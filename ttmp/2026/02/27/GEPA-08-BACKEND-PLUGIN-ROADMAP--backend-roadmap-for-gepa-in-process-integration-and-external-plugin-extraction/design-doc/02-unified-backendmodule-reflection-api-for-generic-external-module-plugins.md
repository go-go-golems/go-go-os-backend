---
Title: Unified BackendModule reflection API for generic external module plugins
Ticket: GEPA-08-BACKEND-PLUGIN-ROADMAP
Status: active
Topics:
    - gepa
    - plugins
    - backend
    - architecture
    - events
    - go-go-os
    - runner
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/inventory_backend_module.go
      Note: Current module route wiring and profile API handler integration.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go
      Note: Module registry composition, middleware definitions, extension schema wiring.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/manifest_endpoint.go
      Note: |-
        Existing /api/os/apps endpoint shape and current manifest fields.
        /api/os/apps manifest surface
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/module.go
      Note: |-
        Current AppBackendModule contract baseline with no reflection hooks.
        Current AppBackendModule baseline
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/routes.go
      Note: |-
        Namespaced route contract and anti-alias policy.
        Namespaced routes and alias constraints
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/pinoweb/middleware_definitions.go
      Note: Existing middleware definition schema contracts via ConfigJSONSchema().
    - Path: ../../../../../../../go-go-os/packages/engine/src/chat/runtime/profileApi.ts
      Note: |-
        Existing frontend discovery APIs for middleware and extension schemas.
        Frontend schema discovery API references
    - Path: ../../../../../../../go-go-os/packages/engine/src/chat/sem/pb/proto/sem/base/log_pb.ts
      Note: Generated event schema artifacts (protobuf->TS).
    - Path: ../../../../../../../go-go-os/packages/engine/src/chat/sem/semRegistry.ts
      Note: Runtime SEM event decode/handler registration surface.
    - Path: cmd/gepa-runner/gepa_plugins_module.go
      Note: Exported plugin API version constants and descriptor constraints.
    - Path: cmd/gepa-runner/plugin_loader.go
      Note: |-
        Optimizer plugin metadata and host context conventions.
        GEPA plugin metadata and runtime behavior
    - Path: pkg/jsbridge/emitter.go
      Note: |-
        GEPA plugin event envelope fields to expose in reflection.
        GEPA event envelopes for reflection mapping
ExternalSources: []
Summary: Detailed architecture research for unifying generic external plugin execution with BackendModule contracts and adding reflection/introspection endpoints for docs, APIs, operations, and event schemas across go-go-os modules.
LastUpdated: 2026-02-27T11:35:00-05:00
WhatFor: Provide a concrete, implementation-ready blueprint for module-level reflection and generic plugin API unification.
WhenToUse: Use when implementing module host evolution, plugin-process integration, and discoverable API/schema contracts in go-go-os.
---


# Unified BackendModule reflection API for generic external module plugins

## Executive summary

This document proposes a unified architecture where:

1. The existing `AppBackendModule` contract remains the backbone of module composition in `go-go-os`.
2. External plugin-process execution is standardized through a **generic** module runtime protocol, not a GEPA-specific protocol.
3. A first-class **reflection surface** is added so each module can expose discoverable:
   - API routes and operation contracts,
   - JSON/protobuf schemas,
   - event type definitions,
   - documentation references,
   - capability metadata.

The central principle is that reflection belongs to the module host layer, not to one module. GEPA benefits from this immediately, but so can inventory-chat and future modules.

The design intentionally preserves existing route and lifecycle invariants:

- namespaced app routes stay under `/api/apps/<app-id>/*`,
- module health remains part of `/api/os/apps` behavior,
- legacy aliases remain forbidden.

The resulting system introduces two complementary planes:

- **execution plane**: request/response/event execution for module operations,
- **reflection plane**: discoverable metadata and schemas for operators, frontend clients, and tooling.

## Problem statement

The current backend module host gives us lifecycle and namespaced routing, but not discoverability depth.

Today, `/api/os/apps` provides only a compact manifest (`app_id`, `name`, `required`, capabilities, health). It does not answer:

- what HTTP endpoints each module exposes,
- what operation payload schema each endpoint expects,
- what event envelopes and event type vocabularies the module emits,
- where documentation for each capability lives,
- whether the module is in-process or plugin-process backed.

For GEPA integration, this becomes a practical bottleneck:

- frontend and tooling need stable contracts,
- plugin extraction requires a generic API boundary,
- onboarding teams need machine-readable introspection, not tribal knowledge.

So the request is correct: phase-2 plugin API should be generic for all modules, and reflection should be explicit and discoverable.

## Scope

### In scope

- Unify plugin-process runtime with `BackendModule` contracts.
- Define generic module runtime protocol for external processes.
- Define reflection model and reflection endpoints.
- Define schemas for APIs, operations, and events.
- Provide phased migration strategy with backward compatibility.
- Include GEPA mapping as one example module.

### Out of scope

- Full implementation in this document (this is research/design).
- Full frontend dynamic module federation.
- Security hardening for untrusted remote plugins (signature/sandbox policy can be follow-up).

## Current-state architecture (evidence-backed)

## 1) Module host contract is clear but reflection-light

Current backend contract:

```go
// go-go-os/go-inventory-chat/internal/backendhost/module.go
type AppBackendModule interface {
    Manifest() AppBackendManifest
    MountRoutes(mux *http.ServeMux) error
    Init(ctx context.Context) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) error
}
```

Evidence:

- `module.go:8-25`

Observation:

- It is strong for execution lifecycle.
- It has no dedicated method for reflection/introspection output.

## 2) Host manifest endpoint is minimal

`RegisterAppsManifestEndpoint` exposes `/api/os/apps` with:

- module identity,
- capability strings,
- health status.

Evidence:

- `manifest_endpoint.go:9-21` defines response shape.
- `manifest_endpoint.go:28-54` serves the endpoint.
- `backendhost_test.go:104-127` validates manifest + health behavior.

Observation:

- Good module liveness summary.
- Insufficient for API/schema/docs discovery.

## 3) Namespaced route invariant is already solved

Route mount policy:

- strict namespacing under `/api/apps/<app-id>`.
- forbidden root legacy aliases.

Evidence:

- `routes.go:29-57` namespaced mount helper.
- `routes.go:59-67` legacy alias guard.
- `App.tsx:33-34` frontend resolvers align to same namespace.

Observation:

- Reflection API must be designed without breaking this invariant.

## 4) Discovery-like pieces already exist in subsystem silos

### Profile and schema discovery in chat runtime

Frontend runtime already expects and consumes:

- `/api/chat/schemas/middlewares`
- `/api/chat/schemas/extensions`

Evidence:

- `profileApi.ts:356-369`
- integration wiring via inventory module `RegisterProfileAPIHandlers(...)`:
  - `inventory_backend_module.go:89-96`

### Middleware definitions already expose JSON schema

Middleware definition API includes:

- `ConfigJSONSchema() map[string]any`

Evidence:

- `middleware_definitions.go:42-44`
- concrete schema payloads in same file (`:79-178`).

### Tool input contracts already carry jsonschema tags

Tool structs include `jsonschema` tags for request fields.

Evidence:

- `tools_inventory.go:26-76`

### Event decode schema assets already exist

SEM decoder imports generated protobuf schemas.

Evidence:

- `semRegistry.ts:1-23`, `:320-413`
- generated schema file comments indicate proto source:
  - `log_pb.ts:1-47`

Observation:

- Reflection primitives exist but are fragmented.
- We need a module-host-level aggregation model.

## 5) GEPA contracts are structured enough for reflection publishing

GEPA already has:

- plugin API version constants and descriptor constraints,
- plugin metadata fields,
- structured event envelope fields,
- explicit runtime modes (`optimizer`, `dataset-generator`) by API version constants.

Evidence:

- `gepa_plugins_module.go:30-131`
- `plugin_loader.go:18-27`, `:50-120`
- `dataset/generator/plugin_loader.go:17-28`, `:55-125`
- `jsbridge/emitter.go:10-23`

Observation:

- GEPA can provide strong reflection documents quickly once reflection plane exists.

## Gap analysis

Current gaps, ordered by impact:

1. **No module-level reflection contract in BackendModule host API**.
2. **No generic plugin-process protocol in host API model** (module-specific framing risk).
3. **No unified event schema discovery endpoint** across modules.
4. **No machine-readable operation catalog** for tooling generation.
5. **No explicit linkage between docs, schema versions, and module capabilities**.

This creates practical issues:

- teams duplicate ad-hoc docs,
- clients handcode endpoint assumptions,
- event/timeline integrations drift over time,
- plugin extraction can accidentally become module-specific.

## Proposed architecture

## Design goals

1. Keep `AppBackendModule` lifecycle compatibility.
2. Add reflection in additive form (no immediate hard break).
3. Make plugin-process protocol generic across modules.
4. Ensure reflection endpoints are stable, cacheable, and versioned.
5. Preserve existing namespaced module routes and `/api/os/apps` semantics.

## High-level model

```mermaid
flowchart TB
  HOST[Backend Host]
  REG[Module Registry]
  REF[Reflection Aggregator]
  EXEC[Execution Router]

  MOD1[Inventory Module]
  MOD2[GEPA Module]
  MODN[Future Module]

  GCLIENT[Generic Plugin Client]
  PPROC[Module Plugin Process]

  HOST --> REG
  HOST --> EXEC
  HOST --> REF

  REG --> MOD1
  REG --> MOD2
  REG --> MODN

  MOD2 --> GCLIENT
  GCLIENT --> PPROC

  REF -->|/api/os/apps| OSAPPS[/OS Apps Manifest/]
  REF -->|/api/os/apps/{id}/reflection| APPREF[/Module Reflection/]
  REF -->|/api/os/reflection/*| GLOBALREF[/Global Reflection Index/]
```

## Unified module contract strategy

We should not replace `AppBackendModule` immediately. We should layer reflection via optional interface assertions.

### Option A (recommended): additive optional interfaces

```go
type AppBackendModule interface {
    Manifest() AppBackendManifest
    MountRoutes(mux *http.ServeMux) error
    Init(ctx context.Context) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health(ctx context.Context) error
}

// Optional
// If a module implements this, host publishes module reflection docs.
type ReflectiveAppBackendModule interface {
    Reflection(ctx context.Context) (AppBackendReflectionDocument, error)
}

// Optional
// If a module executes out-of-process, host can expose runtime mode/health details.
type ExternalRuntimeBackedModule interface {
    RuntimeMode() string // "embedded" | "plugin-process"
    RuntimeDetails(ctx context.Context) (map[string]any, error)
}
```

Why this path:

- zero breakage for existing modules,
- gradual adoption,
- host can enrich `/api/os/apps` with reflection availability flags.

### Option B (hard break): fold reflection into base interface

Rejected for first rollout because all modules must update synchronously.

## Reflection document model

A module reflection doc should be a self-describing bundle with explicit schema version.

### API signature

```go
type AppBackendReflectionDocument struct {
    APIVersion string                      `json:"api_version"` // e.g. os.module-reflection/v1
    AppID      string                      `json:"app_id"`
    Name       string                      `json:"name"`
    Summary    string                      `json:"summary,omitempty"`
    Runtime    ModuleRuntimeDescriptor     `json:"runtime"`
    HTTP       []HTTPRouteDescriptor       `json:"http,omitempty"`
    Operations []OperationDescriptor       `json:"operations,omitempty"`
    Events     []EventSchemaDescriptor     `json:"events,omitempty"`
    Schemas    []NamedSchemaDescriptor     `json:"schemas,omitempty"`
    Docs       []DocumentationDescriptor   `json:"docs,omitempty"`
    Links      map[string]string           `json:"links,omitempty"`
    GeneratedAtMS int64                    `json:"generated_at_ms"`
}
```

```go
type ModuleRuntimeDescriptor struct {
    Mode         string            `json:"mode"` // embedded | plugin-process
    Protocol     string            `json:"protocol,omitempty"` // e.g. os.module-plugin/v1
    Capabilities []string          `json:"capabilities,omitempty"`
    Metadata     map[string]any    `json:"metadata,omitempty"`
}
```

```go
type HTTPRouteDescriptor struct {
    Method      string            `json:"method"`
    Path        string            `json:"path"`
    Description string            `json:"description,omitempty"`
    RequestSchemaRef  string      `json:"request_schema_ref,omitempty"`
    ResponseSchemaRef string      `json:"response_schema_ref,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
}
```

```go
type OperationDescriptor struct {
    Name        string            `json:"name"`
    Description string            `json:"description,omitempty"`
    InputSchemaRef  string        `json:"input_schema_ref,omitempty"`
    OutputSchemaRef string        `json:"output_schema_ref,omitempty"`
    Streaming   bool              `json:"streaming,omitempty"`
}
```

```go
type EventSchemaDescriptor struct {
    Type        string            `json:"type"`
    Summary     string            `json:"summary,omitempty"`
    EnvelopeSchemaRef string      `json:"envelope_schema_ref,omitempty"`
    PayloadSchemaRef  string      `json:"payload_schema_ref,omitempty"`
    Version     string            `json:"version,omitempty"`
}
```

```go
type NamedSchemaDescriptor struct {
    Ref         string            `json:"ref"` // stable ref id, e.g. schema://gepa/run.start.request
    Format      string            `json:"format"` // json-schema | protobuf-descriptor | markdown
    Body        any               `json:"body"`
    Version     string            `json:"version,omitempty"`
}
```

```go
type DocumentationDescriptor struct {
    Slug        string            `json:"slug"`
    Title       string            `json:"title"`
    Format      string            `json:"format"` // markdown | url
    Body        string            `json:"body,omitempty"`
    URL         string            `json:"url,omitempty"`
}
```

## Reflection endpoints

## Existing endpoint (kept)

- `GET /api/os/apps` remains and is extended additively.

Extension fields proposal per app:

- `reflection_available: bool`
- `reflection_path: "/api/os/apps/<app-id>/reflection"`
- `runtime_mode: "embedded" | "plugin-process"`

## New module reflection endpoints

- `GET /api/os/apps/{app_id}/reflection`
- `GET /api/os/apps/{app_id}/reflection/schemas/{ref}`
- `GET /api/os/apps/{app_id}/reflection/docs/{slug}`

## Optional global aggregation endpoints

- `GET /api/os/reflection/index`
- `GET /api/os/reflection/events`
- `GET /api/os/reflection/schemas`

Why global endpoints help:

- docs explorers and CLI tools can fetch one index,
- cross-module event/schema search becomes straightforward,
- frontend can prefetch reflection metadata for plugin cards.

## Example reflection payload (GEPA)

```json
{
  "api_version": "os.module-reflection/v1",
  "app_id": "gepa",
  "name": "GEPA",
  "summary": "GEPA script execution and timeline event APIs",
  "runtime": {
    "mode": "plugin-process",
    "protocol": "os.module-plugin/v1",
    "capabilities": ["scripts", "run", "events", "timeline", "cancel"]
  },
  "http": [
    {
      "method": "GET",
      "path": "/api/apps/gepa/scripts",
      "description": "List discoverable GEPA scripts",
      "response_schema_ref": "schema://gepa.scripts.list.response"
    },
    {
      "method": "POST",
      "path": "/api/apps/gepa/runs",
      "description": "Start a GEPA run",
      "request_schema_ref": "schema://gepa.runs.start.request",
      "response_schema_ref": "schema://gepa.runs.start.response"
    }
  ],
  "operations": [
    {
      "name": "gepa.start_run",
      "description": "Start run operation via generic module runtime",
      "input_schema_ref": "schema://gepa.runs.start.request",
      "output_schema_ref": "schema://gepa.runs.start.response",
      "streaming": true
    }
  ],
  "events": [
    {
      "type": "gepa.plugin.event",
      "summary": "Plugin-originated event emitted during run",
      "envelope_schema_ref": "schema://gepa.events.envelope",
      "payload_schema_ref": "schema://gepa.events.payload"
    }
  ],
  "docs": [
    {
      "slug": "gepa-backend-overview",
      "title": "GEPA Backend Overview",
      "format": "markdown",
      "body": "..."
    }
  ],
  "generated_at_ms": 1772240000000
}
```

## Generic external module plugin runtime API

Reflection and generic runtime should align around one host-level protocol.

### Minimal protocol

```text
service OsModulePluginRuntime {
  rpc Handshake(HandshakeRequest) returns (HandshakeResponse)
  rpc Health(HealthRequest) returns (HealthResponse)
  rpc Invoke(InvokeRequest) returns (InvokeAccepted)
  rpc Stream(InvocationStreamRequest) returns (stream InvocationEvent)
  rpc Cancel(InvocationCancelRequest) returns (InvocationCancelResponse)
}
```

### Key request/response contracts

```text
HandshakeRequest {
  host_version: string
  protocol_version: string
  module_id: string
}

HandshakeResponse {
  protocol_version: string
  module_id: string
  operations: [string]
  capabilities: [string]
  reflection: ReflectionPointer
}
```

```text
InvokeRequest {
  invocation_id: string
  module_id: string
  operation: string
  payload_json: bytes
  context_json: bytes
}

InvocationEvent {
  invocation_id: string
  seq: int64
  timestamp_ms: int64
  event_type: string
  level: string
  message: string
  payload_json: bytes
}
```

### Reflection pointer semantics

`HandshakeResponse.reflection` can provide:

- embedded compact reflection hash/version,
- optional in-band reflection snapshot,
- or endpoint path in plugin process API for reflection fetch.

This prevents stale assumptions when module plugin updates independently.

## Unification with BackendModule lifecycle

Generic runtime layer should not bypass host lifecycle.

### Lifecycle map

- `Init(ctx)`:
  - initialize module stores,
  - initialize plugin client if runtime mode is plugin-process.

- `Start(ctx)`:
  - eager or lazy plugin handshake,
  - background health monitor start.

- `Health(ctx)`:
  - module health + runtime health + protocol compatibility checks.

- `Stop(ctx)`:
  - terminate stream subscriptions,
  - close plugin process/session handles.

### Pseudocode: module + runtime composition

```pseudo
func (m *gepaModule) Init(ctx):
  m.runStore = newRunStore()
  m.eventStore = newEventStore()
  m.timelineStore = newTimelineStore()
  m.runtime = runtimeFactory.Build(m.cfg)
  return nil

func (m *gepaModule) Start(ctx):
  if m.runtime.Mode() == "plugin-process":
    hs = m.runtime.Handshake(ctx)
    validateProtocol(hs)
    m.cachedReflection = mergeHostAndRuntimeReflection(hs)
  return nil

func (m *gepaModule) Health(ctx):
  if err := m.runtime.Health(ctx); err != nil:
    return err
  return nil
```

## Reflection data sources and extraction strategy

Reflection should come from deterministic sources, not manually duplicated docs.

## Source class A: module manifest

- `AppBackendManifest` fields provide app-level identity and capabilities.

## Source class B: route registrations

- module-owned explicit route descriptors should be declared alongside route handlers.
- avoid brittle runtime mux introspection by requiring route descriptor registration.

### Suggested pattern

```go
type RouteRegistrar interface {
    RegisterRoute(method, path, description string, reqRef, respRef string)
}

func (m *gepaModule) MountRoutes(mux *http.ServeMux) error {
    m.routes.RegisterRoute("GET", "/scripts", "List scripts", "", "schema://gepa.scripts.list.response")
    m.routes.RegisterRoute("POST", "/runs", "Start run", "schema://gepa.runs.start.request", "schema://gepa.runs.start.response")
    // actual mux.HandleFunc wiring
    return nil
}
```

## Source class C: schema-producing definitions

- Middleware definitions already expose JSON schema via `ConfigJSONSchema()`.
- Tool structs already carry `jsonschema` tags.
- Event schemas already have generated protobuf descriptor artifacts.

Collect these into module reflection `schemas[]`.

## Source class D: module docs

- Keep markdown docs inside module packages or ticket docs.
- publish selected docs through reflection descriptors.

## Event reflection model

## Why event reflection matters

Without event reflection, timeline consumers guess event shape and drift over time.

## Event reflection layers

1. **Transport envelope schema**
   - fields like `event_type`, `seq`, `timestamp`, `payload`.
2. **Event type registry**
   - known types + semantics + version.
3. **Payload schema per type**
   - JSON schema or protobuf descriptor refs.

## Existing leverage points

- SEM handlers are explicitly registered in `semRegistry.ts` (`registerSem(...)`).
- Generated protobuf schemas exist under `packages/engine/src/chat/sem/pb/proto/...`.
- GEPA event envelope fields are explicit in `jsbridge.Event`.

A reflection generator can derive a first useful event catalog from these sources.

## Reflection consistency and versioning

Reflection must be versioned and cache-aware.

### Rules

- every reflection doc includes `api_version` and `generated_at_ms`.
- schema refs are stable and immutable by version.
- breaking schema changes require new schema ref or version suffix.
- handshake should include reflection hash to detect drift quickly.

### Suggested headers

- `ETag`: reflection hash.
- `Cache-Control`: short max-age + must-revalidate for dev; longer for pinned release builds.

## Phased implementation plan

## Phase R0: contract definition (host-level)

1. Add new reflection DTO types in `internal/backendhost`.
2. Add optional `ReflectiveAppBackendModule` interface.
3. Add host reflection endpoint handlers.
4. Extend `/api/os/apps` additively with reflection metadata fields.

Files likely:

- `internal/backendhost/reflection.go`
- `internal/backendhost/reflection_endpoint.go`
- `internal/backendhost/manifest_endpoint.go` (additive fields)
- `internal/backendhost/backendhost_test.go` (new tests)

## Phase R1: inventory module pilot

1. Implement reflection output for inventory module.
2. Populate:
   - profile endpoints,
   - middleware schema refs,
   - extension schema refs,
   - basic docs entries.
3. Validate frontend can consume reflection for diagnostics panels.

Files likely:

- `cmd/go-go-os-launcher/inventory_backend_module.go`
- `internal/pinoweb/middleware_definitions.go` (reuse schema extraction)

## Phase R2: GEPA reflection and runtime mapping

1. Implement GEPA module reflection document.
2. Expose run/event/timeline API schemas.
3. Publish event type catalog from GEPA envelope model.
4. Include plugin descriptor API versions (`gepa.optimizer/v1`, `gepa.dataset-generator/v1`) in reflection.

Files likely (future module package):

- `internal/gepa/module.go`
- `internal/gepa/reflection.go`
- `internal/gepa/schemas/*.json`

## Phase R3: generic plugin-process runtime client

1. Implement generic `OsModulePluginRuntime` host client.
2. Build module-specific adapters (GEPA first).
3. Ensure reflection merges host-declared and runtime-declared metadata.

Files likely:

- `internal/backendhost/pluginruntime/client.go`
- `internal/backendhost/pluginruntime/protocol.go`
- `internal/gepa/runtime_plugin_adapter.go`

## Phase R4: parity and tooling

1. Add parity tests for embedded vs plugin-process runtime modes.
2. Build simple CLI/dev tool that pulls reflection and renders docs/routes/events tables.
3. Add smoke tests in CI for reflection endpoints.

## Detailed API proposal

## `/api/os/apps` additive response

Current fields remain unchanged. Add optional fields:

```json
{
  "apps": [
    {
      "app_id": "gepa",
      "name": "GEPA",
      "required": false,
      "capabilities": ["scripts", "run", "events", "timeline", "cancel"],
      "healthy": true,
      "reflection_available": true,
      "reflection_path": "/api/os/apps/gepa/reflection",
      "runtime_mode": "plugin-process"
    }
  ]
}
```

## `/api/os/apps/{app_id}/reflection`

```json
{
  "api_version": "os.module-reflection/v1",
  "app_id": "inventory",
  "runtime": { "mode": "embedded" },
  "http": [ ... ],
  "operations": [ ... ],
  "events": [ ... ],
  "schemas": [ ... ],
  "docs": [ ... ]
}
```

## `/api/os/reflection/index`

```json
{
  "api_version": "os.module-reflection-index/v1",
  "apps": [
    {
      "app_id": "inventory",
      "name": "Inventory",
      "reflection_path": "/api/os/apps/inventory/reflection"
    },
    {
      "app_id": "gepa",
      "name": "GEPA",
      "reflection_path": "/api/os/apps/gepa/reflection"
    }
  ]
}
```

## Pseudocode: reflection endpoint wiring

```pseudo
func RegisterReflectionEndpoints(mux, registry):
  mux.HandleFunc("/api/os/apps/{app_id}/reflection", func(w, req):
    appID = pathParam(req, "app_id")
    mod = registry.Get(appID)
    if mod == nil:
      write404()
      return

    reflective, ok = mod.(ReflectiveAppBackendModule)
    if !ok:
      write404("module has no reflection")
      return

    doc, err = reflective.Reflection(req.Context())
    if err != nil:
      write500(err)
      return

    writeJSON(doc)
  )
```

## Pseudocode: host manifest enrichment

```pseudo
for module in registry.Modules():
  manifestDoc = fromManifest(module.Manifest())

  if module implements ReflectiveAppBackendModule:
    manifestDoc.reflection_available = true
    manifestDoc.reflection_path = "/api/os/apps/" + appID + "/reflection"

  if module implements ExternalRuntimeBackedModule:
    manifestDoc.runtime_mode = module.RuntimeMode()
```

## Pseudocode: generic plugin client + module adapter

```pseudo
class PluginProcessModuleRuntimeClient:
  Handshake(moduleID)
  Health(moduleID)
  Invoke(moduleID, operation, payload)
  Stream(invocationID)
  Cancel(invocationID)

class GepaRuntimeAdapter implements GepaRuntime:
  ListScripts(ctx):
    return client.Invoke("gepa", "gepa.list_scripts", {})

  StartRun(ctx, req, sink):
    ack = client.Invoke("gepa", "gepa.start_run", req)
    go streamLoop(ack.invocationID, sink)
    return ack

  CancelRun(ctx, req):
    return client.Cancel(req.invocationID)
```

## Testing strategy

## Unit tests

- Reflection document validation tests.
- Schema ref uniqueness tests.
- Manifest enrichment tests (`reflection_available`, `runtime_mode`).
- Adapter mapping tests (GEPA operation -> generic invoke call).

## Integration tests

- `/api/os/apps` still returns legacy fields and now additive reflection fields.
- `/api/os/apps/{id}/reflection` returns document for reflective modules.
- Non-reflective module reflection request returns 404 or explicit not-supported error.
- `embedded` and `plugin-process` runtime modes return same HTTP behavior for GEPA module.

## Contract tests

- Golden fixtures for reflection docs (inventory and gepa).
- Golden fixtures for event schema listings.
- Golden fixtures for schema refs and operation lists.

## Conformance tests for generic plugin API

- handshake mismatch behavior,
- invoke/stream event order monotonicity,
- cancellation idempotency,
- restart and reconnect semantics.

## Risks and mitigations

- **Risk:** reflection payload becomes stale/manual.
  - **Mitigation:** derive from source code declarations + schema registries where possible; add CI diff checks.

- **Risk:** schema duplication between module docs and reflection docs.
  - **Mitigation:** reflection docs should point to canonical schema refs, not duplicate prose-only schemas.

- **Risk:** module authors skip reflection implementation.
  - **Mitigation:** keep optional for transition, then gate new modules with reflection-required policy.

- **Risk:** generic protocol is over-abstract and slows module teams.
  - **Mitigation:** keep protocol minimal (invoke/stream/cancel/health/handshake) and move module specifics into operations.

- **Risk:** event schema version drift.
  - **Mitigation:** event types and payload schema refs versioned explicitly; parity tests across modes.

## Alternatives considered

1. **Keep GEPA-only plugin protocol and document others later**
   - Rejected: creates technical debt and protocol fragmentation immediately.

2. **Put reflection under module namespace only (`/api/apps/<id>/...`) and skip global endpoints**
   - Partially accepted: module endpoint required; global index still recommended for discoverability.

3. **Embed full OpenAPI per module now**
   - Deferred: useful later, but can start with lighter reflection schema model and add OpenAPI export once routing/DTO extraction stabilizes.

4. **Break AppBackendModule and require reflection now**
   - Rejected for first rollout due migration friction; additive optional interface is safer.

## Open questions

1. Should reflection endpoints live only under `/api/os/apps/{id}/reflection`, or also mirrored under `/api/apps/{id}/api/reflection` for module-local tooling?
2. Should module reflection schemas prefer JSON Schema first, protobuf descriptors first, or both always?
3. Should reflection docs include example payloads as first-class fields or remain external doc links?
4. What is the canonical schema ref format (`schema://...` vs URL path refs)?
5. At what point do we make reflection implementation mandatory for all modules?

## Recommended next actions

1. Approve additive `ReflectiveAppBackendModule` interface strategy.
2. Implement host reflection endpoints and manifest enrichment in `backendhost` package.
3. Pilot with inventory module to verify schema/docs/event publication mechanics.
4. Implement GEPA reflection docs and generic runtime adapter mapping on top.
5. Add reflection parity tests before switching GEPA default runtime to plugin-process.

## Appendix A: Proposed reflection endpoint matrix

| Endpoint | Purpose | Required | Notes |
|---|---|---|---|
| `/api/os/apps` | app health + capability summary | yes | existing endpoint, additive fields only |
| `/api/os/apps/{id}/reflection` | full reflection document | yes (for reflective modules) | host-level canonical module introspection |
| `/api/os/apps/{id}/reflection/schemas/{ref}` | retrieve one schema by ref | optional | useful for large schema bodies |
| `/api/os/apps/{id}/reflection/docs/{slug}` | retrieve one doc entry | optional | supports docs lazy loading |
| `/api/os/reflection/index` | global app reflection index | recommended | tooling-friendly bootstrap |
| `/api/os/reflection/events` | global event catalog | optional | useful for event explorer/debug tools |

## Appendix B: GEPA operation mapping table

| GEPA backend API | Generic operation | Streaming | Primary schema refs |
|---|---|---|---|
| `GET /scripts` | `gepa.list_scripts` | no | `schema://gepa.scripts.list.response` |
| `POST /runs` | `gepa.start_run` | yes | `schema://gepa.runs.start.request`, `schema://gepa.runs.start.response` |
| `GET /runs/{id}` | `gepa.get_run` | no | `schema://gepa.runs.get.response` |
| `GET /runs/{id}/events` | `gepa.stream_events` | yes | `schema://gepa.events.envelope` |
| `GET /runs/{id}/timeline` | `gepa.get_timeline` | no | `schema://gepa.timeline.snapshot` |
| `POST /runs/{id}/cancel` | `gepa.cancel_run` | no | `schema://gepa.runs.cancel.response` |

## Appendix C: Suggested schema refs for initial rollout

- `schema://os.module.reflection.document`
- `schema://os.module.reflection.route`
- `schema://os.module.reflection.operation`
- `schema://os.module.reflection.event`
- `schema://os.module.reflection.schema`
- `schema://gepa.runs.start.request`
- `schema://gepa.runs.start.response`
- `schema://gepa.events.envelope`
- `schema://gepa.timeline.snapshot`

## References

- `go-go-os/go-inventory-chat/internal/backendhost/module.go`
- `go-go-os/go-inventory-chat/internal/backendhost/manifest_endpoint.go`
- `go-go-os/go-inventory-chat/internal/backendhost/routes.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/inventory_backend_module.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go`
- `go-go-os/go-inventory-chat/internal/pinoweb/middleware_definitions.go`
- `go-go-os/packages/engine/src/chat/runtime/profileApi.ts`
- `go-go-os/packages/engine/src/chat/sem/semRegistry.ts`
- `go-go-os/packages/engine/src/chat/sem/pb/proto/sem/base/log_pb.ts`
- `go-go-gepa/pkg/jsbridge/emitter.go`
- `go-go-gepa/cmd/gepa-runner/gepa_plugins_module.go`
- `go-go-gepa/cmd/gepa-runner/plugin_loader.go`

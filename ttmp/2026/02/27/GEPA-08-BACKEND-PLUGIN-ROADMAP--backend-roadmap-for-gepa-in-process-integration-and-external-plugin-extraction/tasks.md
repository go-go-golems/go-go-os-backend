# Tasks

## Done

- [x] Create `GEPA-08-BACKEND-PLUGIN-ROADMAP` ticket workspace.
- [x] Add backend-focused design doc and implementation diary documents.
- [x] Map existing `go-go-os` backend host contracts and lifecycle behavior.
- [x] Map `go-go-gepa` runtime/event APIs relevant to backend integration.
- [x] Produce detailed backend implementation research document (7+ pages equivalent).
- [x] Produce detailed unified `BackendModule` reflection + generic external module runtime research document (7+ pages equivalent).
- [x] Produce Part-1-only internal `BackendModule` integration design doc (excluding generic runtime/event protocol).
- [x] Produce long-form intern-friendly implementation report (10+ pages equivalent) with API reference and tutorials.
- [x] Run `docmgr doctor` validation for the ticket after document updates.
- [x] Upload ticket bundle to reMarkable and verify cloud listing.
- [x] Upload separate reflection-focused v3 bundle to reMarkable and verify cloud listing.
- [x] Upload separate intern-guide-focused v4 bundle to reMarkable and verify cloud listing.

## Phase 1: In-process GEPA backend module

### A. Module scaffolding and wiring

- [x] Create `internal/gepa/module.go` implementing `backendhost.AppBackendModule`.
- [x] Define `Manifest()` with `app_id=gepa`, `name=GEPA`, and backend capability list.
- [x] Add dependency constructor (`NewModuleWithRuntime(...)`) with explicit nil checks.
- [x] Register `gepa` module in `cmd/go-go-os-launcher/main.go` module registry.
- [ ] Ensure lifecycle startup includes GEPA module health validation path.
- [x] Add module-level health implementation that verifies runtime readiness.

### B. API contracts and handlers

- [x] Create backend handler package for `/api/apps/gepa/*`.
- [x] Implement `GET /scripts`.
- [x] Implement `POST /runs`.
- [x] Implement `GET /runs/{run_id}`.
- [x] Implement `GET /runs/{run_id}/events` (SSE).
- [x] Implement `GET /runs/{run_id}/timeline`.
- [x] Implement `POST /runs/{run_id}/cancel`.
- [x] Add common request validation helpers and canonical error responses.
- [x] Add API response DTO structs and JSON encoding tests.

### C. Runtime adapter and run manager

- [x] Define `GepaRuntime` interface for list/start/cancel/health.
- [ ] Implement `InProcessGepaRuntime` using `go-go-gepa` runtime APIs.
- [x] Add script discovery/catalog component with deterministic IDs.
- [x] Add run manager with state machine (`queued/running/completed/failed/canceled`).
- [x] Add cancellation context wiring per run.
- [x] Add run timeout guard support.
- [x] Add controlled concurrency limits (`max concurrent runs`).

### D. Event and timeline pipeline

- [x] Define `RunEvent` normalized envelope type.
- [ ] Implement event translator (`jsbridge.Event` -> `RunEvent`).
- [x] Implement in-memory event store with sequence ordering guarantees.
- [x] Implement in-memory timeline projection store.
- [ ] Implement SSE fanout hub with subscriber lifecycle management.
- [x] Add replay support via `afterSeq` query parameter.
- [x] Ensure terminal events always emit once (`completed/failed/canceled`).

### E. Testing and verification

- [x] Unit test run state transitions including invalid transitions.
- [x] Unit test cancel behavior race scenarios.
- [ ] Unit test translator mapping fidelity.
- [ ] Unit test timeline projection merge/upsert behavior.
- [x] Integration test namespaced route mounting for GEPA module.
- [x] Integration test `/api/os/apps` includes GEPA capabilities and health.
- [x] Integration test start run + SSE stream + terminal status.
- [x] Integration test timeline snapshot endpoint reflects streamed events.
- [x] Integration test cancel endpoint for running and non-running runs.
- [ ] Regression test legacy alias routes remain unmounted.

### F. Developer documentation

- [x] Add backend README section for GEPA module routes and config.
- [x] Add sample curl runbook for each endpoint.
- [ ] Add troubleshooting section for runtime startup failures and stuck runs.

## Current implementation snapshot

- [x] Host-level optional reflection interface added (`ReflectiveAppBackendModule`).
- [x] `/api/os/apps` now includes additive reflection hints for reflective modules.
- [x] `/api/os/apps/{app_id}/reflection` endpoint added with `501` fallback for non-reflective modules.
- [x] Internal GEPA module package added and wired (`internal/gepa` + launcher registry).
- [x] GEPA reflection payload includes APIs/schema refs/docs metadata.
- [x] New GEPA integration tests added in launcher suite.

## Phase 2: External plugin runtime extraction

### G. Runtime abstraction hardening

- [x] Ensure all handlers depend only on `GepaRuntime`, not concrete runtime type.
- [ ] Add runtime mode config switch (`inprocess` vs `plugin-process`).
- [ ] Add dual-runtime test harness running shared API contract suite.

### H. Plugin process manager and adapter

- [ ] Define plugin-process protocol handshake/version capabilities.
- [ ] Implement `PluginProcessGepaRuntime` adapter.
- [ ] Implement plugin process lifecycle manager (spawn, health, restart, stop).
- [ ] Implement plugin event stream bridge -> `RunEvent`.
- [ ] Implement cancel forwarding to plugin process.
- [ ] Add startup diagnostics for plugin process failures.

### I. Phase-2 parity and rollout

- [ ] Run phase-1 API suite against plugin-process runtime mode.
- [ ] Fix parity mismatches in run/event/timeline contracts.
- [ ] Add canary runtime mode rollout instructions.
- [ ] Define switch-over checklist to make plugin-process mode default.

## Acceptance criteria

- [ ] All GEPA backend routes exist only under `/api/apps/gepa/*`.
- [ ] `/api/os/apps` reports GEPA module with capabilities and health status.
- [ ] A run can be started, streamed, observed in timeline, and canceled through backend APIs.
- [ ] Event/timeline schemas remain unchanged between in-process and plugin-process runtime modes.
- [ ] Contract tests pass in CI for both runtime modes.

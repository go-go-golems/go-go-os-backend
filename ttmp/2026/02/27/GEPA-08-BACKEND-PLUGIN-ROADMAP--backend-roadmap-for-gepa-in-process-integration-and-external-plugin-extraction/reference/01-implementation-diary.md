---
Title: Implementation diary
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
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/inventory_backend_module.go
      Note: Concrete module implementation pattern used as reference.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go
      Note: Backend module registry wiring evidence
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/module.go
      Note: Baseline backend module contract used to shape GEPA module design.
    - Path: cmd/gepa-runner/plugin_loader.go
      Note: GEPA optimizer runtime execution and event hook behavior.
    - Path: pkg/jsbridge/call_and_resolve.go
      Note: Promise execution semantics referenced in diary
    - Path: pkg/jsbridge/emitter.go
      Note: GEPA event envelope schema mapped into backend event/timeline contracts.
    - Path: ttmp/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP--backend-roadmap-for-gepa-in-process-integration-and-external-plugin-extraction/design-doc/01-backend-implementation-research-in-process-gepa-module-and-phase-2-extraction.md
      Note: |-
        Primary implementation research document built during this diary.
        Diary tracks construction and delivery of this design doc
        Primary phase-1/phase-2 roadmap captured in this diary
    - Path: ttmp/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP--backend-roadmap-for-gepa-in-process-integration-and-external-plugin-extraction/design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md
      Note: Unified reflection and generic runtime extension addendum captured in step 8
    - Path: ttmp/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP--backend-roadmap-for-gepa-in-process-integration-and-external-plugin-extraction/design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md
      Note: Intern-friendly implementation report and API/tutorial reference created after core phase-1 coding.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/gepa/module.go
      Note: Phase-1 internal GEPA BackendModule implementation and handlers.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/gepa/run_service.go
      Note: In-memory run manager state machine and cancellation wiring.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/gepa/runtime.go
      Note: Runtime interface boundary used by handlers.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/gepa/errors.go
      Note: Shared runtime/domain errors for request handling and adapters.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/gepa/catalog.go
      Note: Script discovery/catalog with deterministic IDs.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go
      Note: GEPA module registration in launcher module registry and new flag.
    - Path: ../../../../../../../go-go-os/go-inventory-chat/internal/backendhost/manifest_endpoint.go
      Note: Added reflection hints and module reflection endpoint.
ExternalSources: []
Summary: Chronological diary for creating the GEPA-08 backend roadmap ticket, research artifact, and delivery package.
LastUpdated: 2026-02-27T14:46:00-05:00
WhatFor: Preserve exact commands, reasoning, and outputs used to build GEPA-08 documentation.
WhenToUse: Use when continuing implementation, validating assumptions, or auditing how backend roadmap decisions were made.
---



# Implementation diary

## Goal

Create a new backend-focused ticket that turns prior GEPA-07 pre-research into implementation-grade backend direction, with detailed tasks and migration strategy for in-process first and plugin-process later.

## Context

User request (latest relevant):

- create a new ticket,
- define backend tasks in detail,
- write a very detailed implementation research document with prose, bullet points, pseudocode, API signatures, code snippets, and diagrams,
- store it in ticket,
- upload to reMarkable.

This diary records that exact flow.

## Step 1: Ticket creation and baseline scaffolding

I created a new backend-specific ticket instead of overloading GEPA-07.

### Commands executed

```bash
cd /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-gepa
docmgr status --summary-only
docmgr ticket list
docmgr ticket create-ticket \
  --ticket GEPA-08-BACKEND-PLUGIN-ROADMAP \
  --title "Backend roadmap for GEPA in-process integration and external plugin extraction" \
  --topics gepa,plugins,backend,architecture,events,go-go-os,runner
docmgr doc add --ticket GEPA-08-BACKEND-PLUGIN-ROADMAP --doc-type design-doc \
  --title "Backend implementation research: in-process GEPA module and phase-2 extraction"
docmgr doc add --ticket GEPA-08-BACKEND-PLUGIN-ROADMAP --doc-type reference \
  --title "Implementation diary"
```

### Why

A dedicated ticket keeps backend implementation planning separate from broader OS integration and frontend considerations.

### Result

- Ticket created under:
  - `ttmp/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP--backend-roadmap-for-gepa-in-process-integration-and-external-plugin-extraction`
- Design doc and reference diary scaffolds created.

## Step 2: Evidence harvest across go-go-os and go-go-gepa

I did a file-backed architecture sweep before writing any recommendations.

### Commands executed (representative)

```bash
rg -n "type AppBackendModule|MountNamespacedRoutes|NewModuleRegistry|/api/os/apps" \
  go-go-os/go-inventory-chat/internal/backendhost \
  go-go-os/go-inventory-chat/cmd/go-go-os-launcher -S

rg -n "type LaunchableAppModule|resolveApiBase|resolveWsBase" \
  go-go-os/packages/desktop-os go-go-os/apps/os-launcher -S

rg -n "PluginContext|CallAndResolve|EventSink|stream-event|loadOptimizerPlugin" \
  go-go-gepa/cmd/gepa-runner go-go-gepa/pkg/jsbridge go-go-gepa/pkg/dataset -S

nl -ba go-go-os/go-inventory-chat/internal/backendhost/module.go | sed -n '1,220p'
nl -ba go-go-os/go-inventory-chat/internal/backendhost/lifecycle.go | sed -n '1,280p'
nl -ba go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go | sed -n '150,280p'
nl -ba go-go-gepa/pkg/jsbridge/emitter.go | sed -n '1,300p'
nl -ba go-go-gepa/pkg/jsbridge/call_and_resolve.go | sed -n '1,260p'
```

### Findings

- Backend host contract and lifecycle in OS are implementation-ready.
- Registration source is still static (`main.go`), which is fine for phase 1.
- GEPA already emits structured events and supports Promise-based runtime behavior.
- Timeline/debug ingestion path in OS engine is mature enough to reuse.

### Why this mattered

This confirmed we do not need to invent a new backend host. We need to implement a GEPA module that conforms to existing host contracts and isolate runtime via interface boundaries.

## Step 3: Design synthesis and long-form doc authoring

I replaced template placeholders with a full implementation research document.

### Authoring actions

- Wrote architecture sections with evidence-backed references.
- Added stable API design for routes and DTOs.
- Added Go interface signatures for runtime and store boundaries.
- Added phase-specific diagrams and pseudocode.
- Added detailed phase 1 and phase 2 implementation and test plan.
- Added risk matrix, alternatives, and open questions.

### Validation

```bash
wc -l design-doc/01-backend-implementation-research-in-process-gepa-module-and-phase-2-extraction.md
```

Output:

- `941` lines, satisfying the requested depth and length target.

## Step 4: Detailed backend task planning

I converted the ticket task file from scaffold to implementation checklist suitable for assignment and tracking.

### Task model used

- Phase 1A-1F for in-process delivery.
- Phase 2G-2I for extraction.
- explicit acceptance criteria at end.

### Why

This lets multiple engineers parallelize work while preserving migration invariants.

## Step 5: Ticket status docs update

I updated ticket metadata docs for onboarding and continuity.

### Updated files

- `index.md`: ticket purpose, scope, current status, key links.
- `changelog.md`: explicit record of design/task/diary deliverables.
- `reference/01-implementation-diary.md`: this chronological trace.

## Step 6: Validation and reMarkable delivery

This step covers required delivery hygiene.

### Validation commands

```bash
cd /home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-gepa
docmgr doctor --ticket GEPA-08-BACKEND-PLUGIN-ROADMAP --stale-after 30
```

### Upload commands

```bash
remarquee status
remarquee cloud account --non-interactive

remarquee upload bundle --dry-run \
  <index.md> \
  <design-doc.md> \
  <diary.md> \
  --name "GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v1" \
  --remote-dir "/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP" \
  --toc-depth 2

remarquee upload bundle \
  <index.md> \
  <design-doc.md> \
  <diary.md> \
  --name "GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v1" \
  --remote-dir "/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP" \
  --toc-depth 2

remarquee cloud ls /ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP --long --non-interactive
```

### Notes

- Dry-run is mandatory in this workflow.
- Bundle upload is preferred so onboarding readers get one PDF with table of contents.
- Validation result:
  - `docmgr doctor` returned all checks passed for `GEPA-08-BACKEND-PLUGIN-ROADMAP`.
- Upload result:
  - artifact uploaded: `GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v1.pdf`
  - remote location: `/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP`
  - cloud listing confirms file presence.

## Step 7: Phase-2 API generalized and v2 delivery

User requested that the external plugin API be generic for all `go-go-os` modules, not GEPA-specific. I revised the phase-2 section accordingly.

### What changed

- Replaced GEPA-specific plugin protocol framing with a generic `OsModulePluginRuntime` host/plugin contract.
- Added explicit mapping layer showing GEPA operations as module-specific operations over the generic API.
- Updated migration plan to include:
  - generic host client (`PluginProcessModuleRuntimeClient`),
  - GEPA adapter (`PluginProcessGepaRuntimeAdapter`).
- Updated phase-2 sequence diagram to show generic client and module process separation.

### Delivery

- Uploaded a separate bundle:
  - `GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v2.pdf`
- Verified remote listing shows both v1 and v2 under:
  - `/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP`

## Step 8: Unified BackendModule reflection research addendum

User requested a deeper follow-up focused on unifying the generic `BackendModule` API with a first-class reflection surface for docs, APIs, operations, and event schemas.

### New document created

- `design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md`

### Research and writing focus

- Keep the runtime API generic for all modules, not GEPA-specific.
- Add reflection contracts that make module docs/schemas discoverable through backend endpoints.
- Define how GEPA maps to generic operation dispatch and generic event envelope contracts.
- Add concrete phased migration and compatibility strategy.

### Commands executed (representative)

```bash
wc -l design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md
sed -n '1,120p' design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md
```

### Result

- Document length: `1006` lines.
- Document includes:
  - prose explanation and rationale,
  - API signatures and pseudocode,
  - endpoint and schema definitions,
  - compatibility matrices,
  - sequence/component diagrams,
  - implementation and test guidance.

## Step 9: Validation and separate v3 reMarkable delivery

After wiring ticket metadata and links, I ran validation and uploaded a separate v3 artifact focused on the new reflection research addendum.

### Commands executed

```bash
docmgr doctor --ticket GEPA-08-BACKEND-PLUGIN-ROADMAP --stale-after 30

remarquee upload bundle --dry-run \
  index.md \
  design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md \
  reference/01-implementation-diary.md \
  --name "GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v3" \
  --remote-dir "/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP" \
  --toc-depth 2

remarquee upload bundle \
  index.md \
  design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md \
  reference/01-implementation-diary.md \
  --name "GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v3" \
  --remote-dir "/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP" \
  --toc-depth 2

remarquee cloud ls /ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP --long --non-interactive
```

### Result

- `docmgr doctor` passed for `GEPA-08-BACKEND-PLUGIN-ROADMAP`.
- v3 uploaded successfully:
  - `GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v3.pdf`
- Cloud listing confirms `v1`, `v2`, and `v3` artifacts are all present.

## Step 10: Part-1-only internal BackendModule design document

User asked for only the internal BackendModule part, explicitly excluding the generic runtime/event protocol work.

### New document created

- `design-doc/03-part-1-internal-backendmodule-integration-only.md`

### Content focus

- Internal `AppBackendModule` integration only.
- Additive reflection interface and endpoint design only.
- Internal route and manifest evolution only.
- Explicit out-of-scope sections for generic runtime and event protocol concerns.

### Result

- Part-1 doc authored and linked from ticket index.
- Ticket bookkeeping updated (`tasks.md`, `changelog.md`, `reference/01-implementation-diary.md`).

## Step 11: Phase-1 implementation task execution in go-go-os

User asked to start implementation task-by-task with commits and a detailed diary. I executed the first three implementation slices in `go-go-os`.

### Slice 1: backend host reflection primitives

Scope:

- Add optional reflection interface at host contract level.
- Add additive reflection hints to `/api/os/apps`.
- Add route `/api/os/apps/{app_id}/reflection`.
- Add backendhost tests for reflection behavior and fallback semantics.

Files touched:

- `go-go-os/go-inventory-chat/internal/backendhost/module.go`
- `go-go-os/go-inventory-chat/internal/backendhost/manifest_endpoint.go`
- `go-go-os/go-inventory-chat/internal/backendhost/backendhost_test.go`

Validation:

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./internal/backendhost -count=1
```

Commit:

- `48763fd` — `backendhost: add optional module reflection endpoint`

### Slice 2: internal GEPA module + launcher wiring

Scope:

- Create new internal module package:
  - `internal/gepa/catalog.go` (script discovery),
  - `internal/gepa/run_service.go` (in-memory run manager),
  - `internal/gepa/schemas.go` (schema documents),
  - `internal/gepa/module.go` (manifest, routes, health, reflection).
- Wire GEPA module into launcher registry (`main.go`).
- Add new launcher integration tests for:
  - GEPA presence in `/api/os/apps`,
  - GEPA reflection endpoint,
  - GEPA scripts endpoint mount.
- Add module unit tests (`internal/gepa/module_test.go`).

Validation commands:

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./internal/backendhost ./internal/gepa -count=1
GOWORK=off go test ./cmd/go-go-os-launcher -run 'Test(OSAppsEndpoint_ListsGEPAModuleReflectionMetadata|GEPAModule_ReflectionAndScriptsEndpoints)$' -count=1
```

Known unrelated failure while running broad launcher suite:

- `TestProfileAPI_CRUDRoutesAreMounted` failed with:
  - `unexpected profile API contract key: registry`
- This is not caused by GEPA module changes and predates this slice's targeted surface.

Commit:

- `9231cb8` — `launcher: wire internal gepa backend module phase 1`

### Slice 3: backend module docs/runbook

Scope:

- Update launcher README with:
  - GEPA module route list,
  - `--gepa-scripts-root` flag,
  - curl runbook for scripts/runs/reflection/schema endpoints.

File touched:

- `go-go-os/go-inventory-chat/README.md`

Commit:

- `7d1c9e7` — `docs: add gepa backend module routes and runbook`

### Current implementation status after Step 11

- Part-1 host reflection API and endpoint are implemented.
- Internal GEPA module is mounted and reachable under `/api/apps/gepa/*`.
- Reflection payload for GEPA is discoverable from `/api/os/apps` and `/api/os/apps/gepa/reflection`.
- Script list/start/get/cancel/schema routes are implemented with in-memory placeholder runtime behavior.
- SSE/timeline routes and real `go-go-gepa` runtime execution are not implemented yet.

## Step 12: Runtime guardrails (timeout + concurrency) and new launcher flags

To advance the task list beyond scaffolding, I implemented two runtime control features in the in-memory GEPA run manager:

- timeout guard (`failed` with `"run timed out"`),
- max-concurrency guard (`429 Too Many Requests` when the limit is reached).

### Files changed

- `go-go-os/go-inventory-chat/internal/gepa/run_service.go`
- `go-go-os/go-inventory-chat/internal/gepa/module.go`
- `go-go-os/go-inventory-chat/internal/gepa/module_test.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main_integration_test.go`
- `go-go-os/go-inventory-chat/README.md`

### Behavior changes

- Added config knobs:
  - `RunTimeout` and `MaxConcurrentRuns` in `gepa.ModuleConfig`.
- Exposed launcher flags:
  - `--gepa-run-timeout-seconds` (default `30`),
  - `--gepa-max-concurrent-runs` (default `4`).
- Run manager now:
  - fails running runs on timeout,
  - returns concurrency-limit error when running count hits cap.
- HTTP handler maps concurrency-limit error to:
  - `429 Too Many Requests`.

### Validation commands

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./internal/gepa ./internal/backendhost -count=1
GOWORK=off go test ./cmd/go-go-os-launcher -run 'Test(OSAppsEndpoint_ListsGEPAModuleReflectionMetadata|GEPAModule_ReflectionAndScriptsEndpoints)$' -count=1
```

### Commit

- `dbe2d60` — `gepa: enforce run timeout and concurrency limits`

## Step 13: Run events SSE and timeline projection endpoints

To advance the remaining Part-1 backend API surface, I implemented run event streaming and timeline projection endpoints on top of the in-memory run manager.

### Scope implemented

- `GET /api/apps/gepa/runs/{run_id}/events`
  - SSE response (`text/event-stream`)
  - supports replay cursor `afterSeq`
  - emits `run.started`, terminal event (`run.completed|run.failed|run.canceled`)
- `GET /api/apps/gepa/runs/{run_id}/timeline`
  - returns structured timeline projection summary:
    - `run_id`, `status`, `last_seq`, `last_event`, `event_count`, `counts`, `events`.

### Storage/model changes

- Added `RunEvent` envelope with ordered sequence ids.
- Added per-run in-memory event log with monotonic `seq`.
- Appended events from run transitions in the run manager:
  - start,
  - complete,
  - fail(timeout),
  - cancel.

### Files changed

- `go-go-os/go-inventory-chat/internal/gepa/run_service.go`
- `go-go-os/go-inventory-chat/internal/gepa/module.go`
- `go-go-os/go-inventory-chat/internal/gepa/schemas.go`
- `go-go-os/go-inventory-chat/internal/gepa/module_test.go`
- `go-go-os/go-inventory-chat/cmd/go-go-os-launcher/main_integration_test.go`
- `go-go-os/go-inventory-chat/README.md`

### Validation commands

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./internal/gepa ./internal/backendhost -count=1
GOWORK=off go test ./cmd/go-go-os-launcher -run 'Test(OSAppsEndpoint_ListsGEPAModuleReflectionMetadata|GEPAModule_ReflectionAndScriptsEndpoints|GEPAModule_RunTimelineAndEventsEndpoints)$' -count=1
```

### Result

- Targeted tests pass for backendhost/gepa and launcher GEPA integration paths.

### Commit

- `36a4765` — `gepa: add run events stream and timeline endpoints`

## Step 14: Cancel endpoint integration coverage (running + terminal)

I added a focused launcher integration test to validate cancel behavior semantics that were still open in the task list.

### Added test

- `TestGEPAModule_CancelEndpointRunningAndTerminalRun`

### Behavior covered

- start a GEPA run from discovered script id,
- cancel immediately while run is active,
- verify first cancel returns `status: canceled`,
- cancel again on already-terminal run,
- verify second cancel stays `status: canceled` and returns success.

### Validation command

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./cmd/go-go-os-launcher -run 'Test(GEPAModule_ReflectionAndScriptsEndpoints|GEPAModule_RunTimelineAndEventsEndpoints|GEPAModule_CancelEndpointRunningAndTerminalRun)$' -count=1
```

### Result

- Targeted launcher integration suite passed.

### Commit

- `1ee7ce3` — `tests: cover gepa cancel endpoint for running and terminal runs`

## Step 15: Runtime interface hardening (`GepaRuntime`) for handler decoupling

I introduced a dedicated runtime interface to decouple HTTP handlers from concrete storage/execution internals. This closes one of the phase-2-hardening prerequisites while staying fully compatible with phase-1 behavior.

### What changed

- Added `GepaRuntime` interface:
  - `ListScripts`, `StartRun`, `GetRun`, `CancelRun`, `ListEvents`.
- Added `InMemoryRuntime` adapter that composes:
  - `ScriptCatalog`,
  - `RunService`.
- Refactored `Module` to hold `runtime GepaRuntime` only.
- Added shared runtime errors (`ErrScriptIDRequired`, `ErrUnknownScriptID`) and reused them in request handling.

### Files changed

- `go-go-os/go-inventory-chat/internal/gepa/runtime.go`
- `go-go-os/go-inventory-chat/internal/gepa/errors.go`
- `go-go-os/go-inventory-chat/internal/gepa/module.go`

### Validation commands

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./internal/gepa ./internal/backendhost -count=1
GOWORK=off go test ./cmd/go-go-os-launcher -run 'Test(GEPAModule_ReflectionAndScriptsEndpoints|GEPAModule_RunTimelineAndEventsEndpoints|GEPAModule_CancelEndpointRunningAndTerminalRun)$' -count=1
```

### Result

- Targeted suites passed after refactor.

### Commit

- `46efc18` — `gepa: introduce runtime interface for handler decoupling`

## Step 16: Run-service transition/race/replay unit tests

I added focused unit coverage at the `InMemoryRunService` level to harden behavior independently from HTTP routes.

### Added test file

- `go-go-os/go-inventory-chat/internal/gepa/run_service_test.go`

### Test coverage added

- state transition flow:
  - `running` -> `completed`,
  - verifies terminal event sequence includes `run.started` then `run.completed`.
- cancel race:
  - concurrent cancel calls on same run,
  - verifies final status is `canceled`,
  - verifies only one terminal cancel event is emitted.
- replay semantics:
  - `Events(afterSeq)` returns only events beyond cursor.

### Validation command

```bash
cd go-go-os/go-inventory-chat
GOWORK=off go test ./internal/gepa ./internal/backendhost -count=1
GOWORK=off go test ./cmd/go-go-os-launcher -run 'Test(GEPAModule_ReflectionAndScriptsEndpoints|GEPAModule_RunTimelineAndEventsEndpoints|GEPAModule_CancelEndpointRunningAndTerminalRun)$' -count=1
```

### Result

- All targeted suites passed.

### Commit

- `29618ff` — `tests: add run-service transition and race coverage`

## Step 17: Intern onboarding report requested (10+ page writeup + reMarkable delivery)

User requested a long-form teaching report that explains:

- what code was implemented,
- how the architecture works,
- concrete API references,
- tutorial examples for new engineers,
- what next step implementation should target.

### New document created

- `design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md`

### Coverage included in the report

- plain-language architecture walkthrough for intern onboarding,
- commit-by-commit change narrative,
- endpoint-by-endpoint API reference,
- runtime/state/event model explanation,
- script discovery and identity rules,
- timeout/concurrency semantics,
- reflection model and usage,
- multiple tutorial runbooks (`curl` + test commands),
- debugging playbook,
- extension guidance and reviewer checklists,
- next-step roadmap for real `go-go-gepa` runtime integration.

### Size/depth verification

- line count: `1511`
- word count: `5094`

### Validation and delivery flow (completed)

Commands executed:

```bash
docmgr doctor --ticket GEPA-08-BACKEND-PLUGIN-ROADMAP --stale-after 30

remarquee upload bundle --dry-run \
  index.md \
  design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md \
  reference/01-implementation-diary.md \
  --name "GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v4-intern-guide" \
  --remote-dir "/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP" \
  --toc-depth 2

remarquee upload bundle \
  index.md \
  design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md \
  reference/01-implementation-diary.md \
  --name "GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v4-intern-guide" \
  --remote-dir "/ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP" \
  --toc-depth 2

remarquee cloud ls /ai/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP --long --non-interactive
```

Result:

- `docmgr doctor` passed.
- Upload succeeded:
  - `GEPA-08-BACKEND-PLUGIN-ROADMAP-backend-research-2026-02-27-v4-intern-guide.pdf`.
- Cloud listing confirms `v1`, `v2`, `v3`, and `v4-intern-guide` artifacts.

## Quick reference

## Ticket path

- `/home/manuel/workspaces/2026-02-22/add-gepa-optimizer/go-go-gepa/ttmp/2026/02/27/GEPA-08-BACKEND-PLUGIN-ROADMAP--backend-roadmap-for-gepa-in-process-integration-and-external-plugin-extraction`

## Primary documents

- `design-doc/01-backend-implementation-research-in-process-gepa-module-and-phase-2-extraction.md`
- `design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md`

## Key implementation recommendation

- Build in-process GEPA backend module first behind `GepaRuntime` interface.
- Freeze HTTP/event/timeline contracts early.
- Extract runtime execution to plugin process later by swapping runtime adapter only.

## Usage examples

### Example continuation plan for first implementation PR

1. Implement module scaffolding and route handlers with mock runtime.
2. Add integration tests for route mount and `/api/os/apps` capability visibility.
3. Merge once contract tests pass, before wiring in full runtime execution.

### Example review checklist for next engineer

1. Confirm route namespace invariants (`/api/apps/gepa/*` only).
2. Confirm run/event schema matches design doc exactly.
3. Confirm run cancel and terminal events are deterministic.
4. Confirm API behavior remains unchanged when runtime mode changes.

## Related

- `../design-doc/01-backend-implementation-research-in-process-gepa-module-and-phase-2-extraction.md`
- `../design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md`
- `../design-doc/03-part-1-internal-backendmodule-integration-only.md`
- `../design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md`
- `../tasks.md`
- `../changelog.md`

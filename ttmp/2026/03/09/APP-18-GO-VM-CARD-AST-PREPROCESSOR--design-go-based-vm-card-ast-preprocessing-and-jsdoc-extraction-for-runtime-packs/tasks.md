# Tasks

## Ticket Setup

- [x] Create APP-18 ticket workspace
- [x] Add APP-18 design doc and implementation diary
- [x] Relate the key `go-go-goja`, `wesen-os`, and runtime-pack files to the ticket docs
- [x] Run `docmgr doctor --ticket APP-18-GO-VM-CARD-AST-PREPROCESSOR --stale-after 30`

## Analysis

- [x] Audit `go-go-goja/pkg/jsparse` tree-sitter APIs relevant to VM card structure extraction
- [x] Audit `go-go-goja/pkg/jsdoc` extraction, model, and batch APIs relevant to pack/card docs
- [x] Audit current `wesen-os` VM bundle and runtime-pack seams that would consume generated metadata
- [x] Explain why a Go-native preprocessor is preferable to a Node/Vite-only manifest generator for this system

## Design

- [x] Define the target build pipeline for scanning VM card source and emitting generated artifacts
- [x] Define the authoring conventions for pack docs, DSL/API docs, and per-card docs
- [x] Decide how card docs fit the current `pkg/jsdoc/model` shape
- [x] Define output artifacts for runtime use, debugger/source use, and docs browsing/export
- [x] Define invocation points for local dev, CI, and package/app builds
- [x] Define recommended package and command layout for the Go tool
- [x] Update the design so the CLI lives in `go-go-os-backend` and imports `github.com/go-go-golems/go-go-goja` without modifying it

## Implementation Guide

- [x] Write a detailed intern guide with prose, diagrams, pseudocode, API sketches, and file references
- [x] Add a phase-by-phase implementation plan with review checkpoints and rollback points
- [x] Record the research step in the diary and changelog

## Implementation Phase 1: Preprocessable Kanban Source Layout

- [x] Create a dedicated `os-launcher` VM metadata source tree for `kanban.v1`
- [x] Add explicit pack-level docs sources for the Kanban DSL/API surface
- [x] Split the three Kanban demo cards into per-card source files that preserve exact authored source
- [x] Define one constrained sentinel format for card metadata that the Go parser can extract deterministically
- [x] Keep the runtime-facing bundle behavior intact while introducing the new metadata source of truth

## Implementation Phase 2: Backend `vmmeta` Command

- [x] Add `github.com/go-go-golems/go-go-goja` as an imported dependency in `go-go-os-backend`
- [x] Add a Cobra-based `vmmeta generate` command to `go-go-os-backend`
- [x] Implement manifest models for cards, packs, docs, and generated outputs
- [x] Implement directory scanning and file loading for card and docs sources
- [x] Emit deterministic JSON output for runtime/debugger/build consumption
- [x] Emit a generated TypeScript module wrapper consumable from `go-go-os-frontend`

## Implementation Phase 3: Structural Extraction

- [x] Reuse `pkg/jsparse` to parse card source files into tree-sitter snapshots
- [x] Extract `cardId`, `packId`, and source file identity from the constrained sentinel format
- [x] Extract handler names from the authored card definition
- [x] Preserve the exact source string for each card in the generated output
- [x] Fail loudly on malformed or ambiguous card metadata declarations

## Implementation Phase 4: Documentation Extraction

- [x] Reuse `pkg/jsdoc` to parse pack docs files and card docs files
- [x] Represent per-card docs in the current `pkg/jsdoc/model` shape without extending upstream packages
- [x] Merge pack docs and card docs into a deterministic generated docs artifact
- [x] Ensure generated docs retain source file and line metadata for debugger/docs tooling

## Implementation Phase 5: Frontend Wiring

- [x] Add an `os-launcher` script that invokes the backend-owned generator before frontend builds
- [x] Check in the generated outputs needed by `os-launcher`
- [x] Add a small frontend-facing helper that imports the generated manifest and docs outputs
- [x] Add a frontend test proving the generated manifest is consumed from `os-launcher` runtime metadata helpers

## Implementation Phase 6: Validation And Hardening

- [x] Add backend unit tests for valid card extraction
- [x] Add backend unit tests for malformed card metadata and malformed docs
- [x] Add a determinism test for stable generated output ordering
- [x] Add a frontend test proving the generated manifest matches the Kanban demo cards
- [x] Run the targeted validation commands that currently pass for the completed slices

## Deferred Follow-up

- [ ] After `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP`, show generated built-in VM card source and docs inside `Stacks & Cards`

---
Title: Backend roadmap for GEPA in-process integration and external plugin extraction
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
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Backend-only execution roadmap for integrating GEPA into go-go-os first in-process, then extracting runtime execution behind a plugin process adapter while preserving stable OS-facing APIs.
LastUpdated: 2026-02-27T14:47:00-05:00
WhatFor: Coordinate implementation-ready backend architecture, tasks, and operational guidance for GEPA OS integration.
WhenToUse: Read this first when implementing or reviewing GEPA backend module work for go-go-os.
---

# Backend roadmap for GEPA in-process integration and external plugin extraction

## Overview

This ticket is the backend implementation blueprint for GEPA integration into `go-go-os`.

Current recommendation:

- **Phase 1:** integrate GEPA runtime in-process as a normal backend module (`app_id=gepa`).
- **Phase 2:** move runtime execution out-of-process behind a plugin framework adapter while keeping backend HTTP contracts unchanged.

This ticket is intentionally backend-scoped and does not require frontend dynamic plugin federation to begin.

## Deliverables in this ticket

- Detailed implementation research/design doc with API contracts, pseudocode, diagrams, and migration strategy.
- Detailed module reflection/unified host contract research doc covering generic `BackendModule` introspection and plugin runtime discovery APIs.
- Part-1-only internal `BackendModule` implementation design doc excluding generic runtime/event protocol concerns.
- Long-form intern onboarding implementation report with API reference, tutorials, and next-step roadmap.
- Granular backend task list broken down by milestone.
- Chronological implementation diary for reproducibility and onboarding.

## Key links

- Design doc:
  - `design-doc/01-backend-implementation-research-in-process-gepa-module-and-phase-2-extraction.md`
  - `design-doc/02-unified-backendmodule-reflection-api-for-generic-external-module-plugins.md`
  - `design-doc/03-part-1-internal-backendmodule-integration-only.md`
  - `design-doc/04-phase-1-implementation-report-and-intern-onboarding-guide.md`
- Diary:
  - `reference/01-implementation-diary.md`
- Task tracker:
  - `tasks.md`
- Changelog:
  - `changelog.md`

## Status

Current status: **active**

Progress snapshot:

- pre-implementation research complete,
- implementation tasks defined,
- reflection/unified host research addendum complete,
- phase-1 implementation started with 8 commits in `go-go-os`,
- validation + reMarkable delivery complete (v1, v2, v3, v4-intern-guide),
- phase-1 coding in progress.

## Structure

- `design-doc/` backend architecture and implementation references
- `reference/` chronological work diary and operational notes
- `scripts/` ticket-local experiments and helper scripts (if added)

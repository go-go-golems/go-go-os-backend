---
Title: Extract go-go-os backend into standalone repository
Ticket: GEPA-31-EXTRACT-BACKEND
Status: active
Topics:
    - go-go-os
    - go
    - migration
    - architecture
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-gepa/ttmp/2026/03/01/GEPA-31-EXTRACT-BACKEND--extract-go-go-os-backend-into-standalone-repository/design-doc/01-backend-extraction-migration-plan-go-go-os-go-go-os-backend.md
      Note: |-
        Primary migration design plan
        Plan and implementation status
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-gepa/ttmp/2026/03/01/GEPA-31-EXTRACT-BACKEND--extract-go-go-os-backend-into-standalone-repository/reference/01-investigation-diary.md
      Note: |-
        Chronological evidence diary
        Detailed chronological implementation log
    - Path: workspaces/2026-02-22/add-gepa-optimizer/go-go-gepa/ttmp/2026/03/01/GEPA-31-EXTRACT-BACKEND--extract-go-go-os-backend-into-standalone-repository/tasks.md
      Note: |-
        Execution checklist
        Implementation task checklist
ExternalSources: []
Summary: Ticket workspace for planning the backendhost extraction from go-go-os into go-go-os-backend with history preservation and downstream rewiring.
LastUpdated: 2026-03-01T11:20:00-05:00
WhatFor: Coordinate and track the repository split plan and delivery artifacts.
WhenToUse: Use when implementing or reviewing GEPA-31 extraction work.
---



# Extract go-go-os backend into standalone repository

## Overview

This ticket captures the implementation plan to move `go-go-os/go-go-os/*` into `go-go-os-backend/*`, preserve history, and update all downstream imports and module wiring.

## Key Links

- Design doc: `design-doc/01-backend-extraction-migration-plan-go-go-os-go-go-os-backend.md`
- Diary: `reference/01-investigation-diary.md`
- Task tracker: `tasks.md`
- Changelog: `changelog.md`

## Status

Current status: **active**

## Topics

- go-go-os
- go
- migration
- architecture
- backend

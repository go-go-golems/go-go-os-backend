---
Title: Design Go-based VM card AST preprocessing and jsdoc extraction for runtime packs
Ticket: APP-18-GO-VM-CARD-AST-PREPROCESSOR
Status: completed
Topics:
    - architecture
    - frontend
    - hypercard
    - wesen-os
    - goja
    - tooling
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-goja/pkg/jsparse/treesitter.go
      Note: Existing Go tree-sitter parser that should power VM card structural extraction
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-goja/pkg/jsdoc/extract/extract.go
      Note: Existing jsdocex-derived extraction engine for package, symbol, example, and prose docs
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-goja/pkg/jsdoc/batch/batch.go
      Note: Existing batch pipeline that can be reused for multi-file pack documentation extraction
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-goja/pkg/jsdoc/model/model.go
      Note: Current doc model that informs whether cards are represented as symbols or need a new model layer
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/go-go-goja/cmd/goja-jsdoc/doc/01-jsdoc-system.md
      Note: Existing intern-facing documentation for jsdoc extraction that this ticket builds on
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-backend/cmd/go-go-os-backend/main.go
      Note: Backend CLI home where the new preprocessor command should live
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-backend/go.mod
      Note: Backend module that should import github.com/go-go-golems/go-go-goja without modifying that upstream repo
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/apps/os-launcher/src/domain/pluginBundle.vm.js
      Note: Current inline VM card bundle showing why per-card file splitting and preprocessing are needed
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/apps/os-launcher/src/domain/pluginBundle.authoring.d.ts
      Note: Current VM authoring surface that could later be complemented by generated metadata and docs
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-frontend/packages/hypercard-runtime/src/runtime-packs/runtimePackRegistry.tsx
      Note: Current runtime-pack registration surface that will eventually consume richer generated metadata
    - Path: /home/manuel/workspaces/2026-03-02/os-openai-app-server/wesen-os/workspace-links/go-go-os-frontend/packages/hypercard-runtime/src/runtime-packs/kanbanV1Pack.tsx
      Note: Concrete first runtime pack whose DSL and renderer docs should be extractable
ExternalSources: []
Summary: Research and design ticket for a Go-based preprocessing pipeline whose CLI lives in go-go-os-backend, imports parser and jsdoc libraries from github.com/go-go-golems/go-go-goja, and emits generated manifests for runtime-pack builds.
LastUpdated: 2026-03-10T23:25:00-04:00
WhatFor: Use this ticket to design the future Go-native build pipeline for VM card metadata and documentation extraction so runtime-pack authoring can stay in JS while structural and prose metadata are generated in a deterministic Go step.
WhenToUse: Use when implementing or reviewing the Go-based VM card preprocessor, when deciding how runtime packs should expose DSL docs and per-card docs, or when onboarding an intern to the relevant go-go-goja and wesen-os integration seams.
---

# Design Go-based VM card AST preprocessing and jsdoc extraction for runtime packs

## Overview

The next step after the `kanban.v1` runtime-pack work is build-time metadata. We now have real VM-authored cards, but we do not have a disciplined way to extract:

- structural card metadata
- exact card source strings
- pack-level DSL/API documentation
- per-card documentation

The user explicitly wants this implemented in Go, not Node. The relevant local foundation already exists in `go-go-goja`, but the CLI itself should live in `go-go-os-backend`:

- `github.com/go-go-golems/go-go-goja/pkg/jsparse` provides tree-sitter-based JavaScript parsing
- `github.com/go-go-golems/go-go-goja/pkg/jsdoc` provides the migrated `jsdocex` extraction pipeline
- `go-go-os-backend/cmd/go-go-os-backend` is the right place to expose the preprocessor as a project-owned CLI

APP-18 is the research and design ticket for turning those pieces into a Go-native preprocessor that future builds can run before Vite/package compilation. The likely output is one or more generated manifest artifacts that frontend/runtime code can consume without hand-maintaining parallel metadata.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **completed**

Current scope:

- audit the relevant `go-go-goja` parser and jsdoc packages
- define a Go-native VM card preprocessing architecture
- define how pack docs and per-card docs should be authored and extracted
- define generated artifact formats and build invocation points
- provide a detailed implementation guide, task plan, and diary for later execution
- stop at generated source/docs artifacts and defer debugger display work to `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP`

Closure note:

- APP-18 is closed as the backend/frontend generation slice
- the next visible step, showing generated built-in card source inside `Stacks & Cards`, is explicitly deferred until `APP-17-HYPERCARD-RUNTIME-DEBUG-BOOTSTRAP` moves that debugger into `hypercard-runtime`

## Topics

- architecture
- frontend
- hypercard
- wesen-os
- goja
- tooling

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts

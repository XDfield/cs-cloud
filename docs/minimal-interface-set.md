# Minimal Interface Set

## Goal

This document defines the minimal interface set that `cs-cloud` should expose for the current `app-ai-native` migration path.

The design goal is to make `cs-cloud` an ACP-compatible runtime/control adapter rather than a product-level workspace backend.

Two principles drive this interface split:

- `workspace` is a cloud product concept, not a core `cs-cloud` protocol concept.
- `worktree` is a device/runtime implementation detail, not a core `cs-cloud` protocol concept.

So the core interface should be organized around:

- runtime target
- capabilities
- conversation lifecycle
- event stream
- human-in-the-loop interaction

Not around:

- workspace CRUD
- project CRUD
- worktree CRUD

## Non-goals

The following should not be treated as first-class `cs-cloud` protocol objects:

- workspace
- project
- worktree
- provider auth/connect flows

These may still exist in product-specific cloud APIs or device-specific runtime implementations, but should not define the main `cs-cloud` SDK shape.

## Interface Families

The current `app-ai-native` usage can be split into two families.

### 1. Global Control / Configuration Surface

This family is not "global config" in the old `global/config` sense.
It means runtime discovery, environment capability inspection, and target-level control.

Recommended capability groups:

- `runtime.health`
- `runtime.target.context`
- `runtime.model.capabilities`
- `runtime.file.*`
- `runtime.find.*`
- `runtime.mcp.*`
- `runtime.lsp.*`
- `runtime.vcs.*`
- `runtime.terminal.*`
- `runtime.instance.dispose`

These are runtime/control APIs, not conversation APIs.

### 2. ACP Conversation Flow Surface

This family is the core ACP-compatible interaction surface.

Recommended capability groups:

- `conversation.create`
- `conversation.get`
- `conversation.list`
- `conversation.update`
- `conversation.delete`
- `conversation.abort`
- `conversation.revert`
- `conversation.unrevert`
- `conversation.messages`
- `conversation.prompt`
- `conversation.command`
- `conversation.shell`
- `conversation.diff`
- `conversation.todo`
- `interaction.permission.list`
- `interaction.permission.respond`
- `interaction.question.list`
- `interaction.question.reply`
- `interaction.question.reject`
- `event.stream`

This family should be the main focus of ACP compatibility.

## What `cs-cloud` Should Not Own

### Workspace

`workspace` should stay in a higher-level product/cloud API.

Examples of workspace concerns:

- workspace list
- workspace detail
- workspace default selection
- directory collection under a workspace
- device-to-workspace binding
- workspace UI metadata such as name, icon, ordering

These can be used by the product shell that hosts `app-ai-native`, but they should not define the `cs-cloud` runtime SDK itself.

### Worktree

`worktree` should not be a first-class SDK concept.

If isolated execution environments are still needed, they should be exposed as neutral runtime semantics, such as:

- `environment.create`
- `environment.dispose`
- `conversation.environment`

But even these should be optional, and not required for the minimal ACP path.

## Current `app-ai-native` Consumption Mapping

The current app still mixes cloud product APIs and device runtime APIs. The target `cs-cloud` split should be based on what is actually consumed today.

### Runtime / Control APIs Still Needed

- `health`
  - current source: `sdk.global.health()`
  - role: runtime reachability

- `path`
  - current source: `sdk.path.get()`
  - role: target directory context, home/worktree/directory metadata
  - note: directory-scoped only; no separate global path needed

- `app.agents`
  - current source: `sdk.app.agents()`
  - role: available execution agents for the current target

- `provider/capabilities`
  - current source: direct fetch in `global-sync/bootstrap.ts`
  - role: connected model capabilities only
  - note: this should eventually be renamed to a model-centric capability interface

- `file.*`
  - current role: file browsing, reading, reviewing, patch navigation

- `find.*`
  - current role: file search and directory search for prompt/file dialogs

- `mcp.status`, `mcp.connect`, `mcp.disconnect`
  - current role: tool/runtime integration state
  - note: lazy-loadable, not bootstrap-critical

- `lsp.status`
  - current role: runtime enhancement state
  - note: lazy-loadable; keep event refresh path

- `vcs.get`
  - current role: branch display in session entry UI
  - note: lazy-loadable, not bootstrap-critical

- `pty.*`
  - current role: terminal runtime

- `instance.dispose`
  - current role: cleanup for target runtime instances

### Conversation Flow APIs Still Needed

- `session.create`
- `session.get`
- `session.list`
- `session.messages`
- `session.update`
- `session.delete`
- `session.abort`
- `session.revert`
- `session.unrevert`
- `session.summarize`
- `session.command`
- `session.shell`
- `session.promptAsync`
- `session.diff`
- `session.todo`

These are the current session-level APIs that should become the basis for a future `conversation.*` surface.

### Human-in-the-loop APIs Still Needed

- `permission.list`
- `permission.respond`
- `question.list`
- `question.reply`
- `question.reject`

These are not optional if we want the UI to restore pending blocked state when re-entering a running or paused conversation.

### Event Stream

Current UI behavior still relies heavily on event-driven state repair and incremental updates.

The event stream needs to cover at least:

- message deltas / part updates
- session status changes
- permission requested / resolved
- question asked / replied / rejected
- lsp updated
- runtime disconnect / disposed events

This should be modeled as a first-class ACP-compatible event channel.

## Proposed Minimal `cs-cloud` Surface

接口定义已拆分至独立文档，本文仅保留设计原则与边界划分：

- **Runtime / Control Surface** → 见 [`runtime-control-api.md`](runtime-control-api.md)
- **ACP Conversation Flow Surface** → 见 [`acp-agent-integration.md`](acp-agent-integration.md) 的 "RESTful API Surface" 章节

## Naming Direction

To align with ACP semantics, the following naming shifts are recommended.

Current names can remain temporarily as compatibility aliases, but new SDK design should prefer the target names.

- `session.*` -> `conversation.*`
- `provider/capabilities` -> `runtime.model.capabilities.list`
- `path.get` -> `runtime.target.context`
- `permission.*` / `question.*` -> `interaction.*`

## Simplification Notes

### Provider

The old provider payload was too heavy because it mixed:

- provider catalog
- connect/auth metadata
- provider config data
- model capability data

The new direction should keep only:

- connected providers
- default model per provider
- model capabilities needed by selection and context calculation

This should be treated as a model capability surface, not a provider management API.

### Path

Only directory-scoped target context is needed.

No separate global path API is needed if the app is already operating inside a selected runtime target.

### Session List

Conversation list is navigation data, not content data.

The runtime API may still provide a list endpoint, but UI ownership should stay in the navigation layer, not the content area.

### MCP / LSP / VCS

These should be lazy-loadable capability/status surfaces.

They do not need to be bootstrap-critical.

## Suggested Next Step

The next document should define request and response shapes for the following minimal targets:

- `runtime.target.context`
- `runtime.model.capabilities.list`
- `conversation.prompt`
- `conversation.messages`
- `interaction.permission.respond`
- `interaction.question.reply`
- `event.stream`

Those are the most valuable interfaces to stabilize first for the new `cs-cloud` SDK.

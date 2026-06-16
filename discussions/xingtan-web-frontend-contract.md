# Xingtan Web Frontend / multi-agent collaboration room contract

Status: product/architecture design note
Related issue: Lingtai-AI/lingtai#167

## Purpose

`Xingtan` (杏坛) started as the TypeScript `agent-chat-room` exam project, but it
proved a product abstraction LingTai may want: a room-oriented web collaboration
surface where humans and multiple agents share a conversation ledger, visible
receipts, participant state, and adapter-backed execution.

This note captures the boundary that should hold if Xingtan becomes a LingTai
Web Frontend / multi-agent collaboration layer. It is not an implementation and
should not by itself close Lingtai-AI/lingtai#167.

## Positioning relative to existing LingTai surfaces

LingTai already has three different layers that must not be conflated:

- **Kernel / agent runtime:** owns agent lifecycle, internal mailbox/email,
  identity, memory/molt, daemon/avatar, MCP/addon wiring, permissions, and durable
  state.
- **TUI:** a local operator interface for project setup, agent creation, direct
  operations, preset/MCP management, and focused terminal workflows.
- **Portal:** a visualization/replay surface over the existing `.lingtai/`
  network, topology, status, lifecycle, and history.

Xingtan should be treated as a fourth, room-oriented collaboration surface:

- it may visualize rooms, participants, messages, receipts, task/PR cards, and
  adapter activity;
- it may send room messages into existing runtime channels through thin adapters;
- it may aggregate logs/traces/replies for humans;
- it must not fork, duplicate, or silently own the kernel runtime.

The key product question remains open: Xingtan can live inside this repository,
inside the existing portal, or as a separate package/repo. This document defines
boundary conditions that apply regardless of placement.

## Ownership boundary

### Kernel / agent runtime continues to own

- agent creation, wake/sleep/suspend/CPR and lifecycle authority;
- internal `.lingtai/` mailbox/email and peer communication semantics;
- context molt, pad, knowledge, skills, soul flow, daemon, avatar;
- MCP/addon wiring and channel-specific delivery semantics;
- authority checks for privileged actions;
- the canonical runtime status for a LingTai agent.

### Xingtan / Web Frontend may own

- room and group visualization;
- participant registry entries that reference LingTai agents or external harness
  sessions;
- shared message ledger and receipt/status timeline for the room;
- human-facing routing UI: who should receive or respond to a room message;
- adapter configuration screens for LingTai agents and external coding harnesses;
- aggregation of replies, traces, logs, and PR/task cards;
- collaboration-state UX such as owner/reviewer/merge-gate visibility.

### Explicit non-ownership

Xingtan must not:

- kill, suspend, CPR, clear, or otherwise manage agent lifecycle without going
  through the same explicit permission/confirmation gates as TUI/runtime;
- treat a participant record as a process owner;
- duplicate kernel mailbox semantics in a way that can diverge from `.lingtai/`;
- expose private pad/knowledge/skills contents by default;
- blur mock participants, command-shell adapters, external sessions, and real
  LingTai agents.

## Core domain model

### Room

A room is a human-visible collaboration context. It has:

- id, display name, description;
- membership list;
- policy knobs (broadcast default, silent/lurker participants, hop limits,
  external sharing, transcript visibility);
- message ledger;
- receipt timeline;
- optional task/PR/issue associations.

### Participant

A participant is a room-visible actor reference, not necessarily a live runtime
process.

```ts
type Participant = {
  id: string;
  displayName: string;
  harnessType: 'lingtai' | 'codex' | 'claude-code' | 'opencode' | 'command' | 'mock';
  externalSessionId?: string;   // opaque reference owned by the harness/runtime
  mailboxAddress?: string;      // for LingTai agent adapters, if safe to expose
  adapterConfigRef?: string;    // config reference, not raw secrets
  capabilities?: string[];
  enabled: boolean;
  roomIds: string[];
  lastError?: string;
};
```

Rules:

- `externalSessionId`, mailbox address, and CLI session id are references only.
- Web UI must label mock/real/command/external/LingTai participants distinctly.
- A disabled participant can still appear in history; new receipts targeting it
  should become `blocked`, not silently disappear.

### Message

A message is an entry in the shared room ledger.

Minimum fields:

- id, room id, author participant/human reference;
- body / attachments / structured metadata;
- source (`human`, `agent`, `adapter`, `system`);
- created timestamp;
- attention metadata (`@mention`, handoff, ask-all, direct reply target);
- parent/correlation ids when mapped from an external runtime.

### Receipt

A receipt is the routing/accounting record that says whether a participant should
process a message and what happened.

Recommended states:

- `pending`: waiting to be claimed;
- `claimed`: a runtime/adapter has accepted work;
- `delivered`: processing completed and a reply or terminal result was recorded;
- `failed`: adapter/runtime error;
- `dropped`: hop-limit, cascade guard, dedupe, or policy drop;
- `blocked`: participant disabled, room dissolved, permission denied, or policy
  block;
- `stale`: claimed too long without heartbeat/progress and eligible for recovery.

Receipts are essential debugging UI. A room that only shows messages cannot answer
"why did the agent not reply?".

## Routing semantics

### Human room messages

When a human posts a normal room message, the default should be room broadcast to
enabled participants that are configured to listen. `@mention` should be attention
metadata, not the only delivery mechanism.

Default flow:

1. Human message is appended to the room ledger.
2. Receipts are created for enabled room participants according to room policy.
3. `@mention` / handoff metadata adjusts priority and display, but does not have
   to be the only routing condition.
4. Disabled or policy-blocked participants receive `blocked` receipts for audit.

### Agent replies

Agent replies should not automatically fan out indefinitely to every participant.
They should use explicit handoff/mention/reply semantics or room policy to avoid
cascade loops.

Required safeguards:

- hop/cascade limit;
- dedupe of repeated adapter output;
- clear distinction between a human-originated room broadcast and an agent-origin
  reply;
- visible `dropped` receipts when cascade guards suppress delivery.

## Adapter principles

Adapters should be thin. They translate between a room receipt and an existing
runtime/harness interface; they should not own the room scheduler.

### LingTai agent adapter

Preferred responsibilities:

- map a room receipt to an existing LingTai communication path (mailbox/internal
  email, a future explicit room protocol, or a safe API surface);
- include enough correlation metadata to map replies back into the room ledger;
- observe existing lifecycle/permission boundaries;
- never copy private pad/knowledge/skills into the room unless explicitly
  requested and authorized.

Open design question: whether the adapter should use internal email, LICC, HTTP
API, or a new room protocol. The answer affects permission, wake, and transcript
semantics and should be decided before implementation.

### External harness adapters

External adapters (Codex, Claude Code, OpenCode, command shell, mock) should:

- identify the harness type and session reference;
- write progress and terminal results back to receipts/messages;
- finalize errors as `failed` rather than leaving receipts permanently claimed;
- avoid storing raw secrets or command invocations in the UI by default;
- mark mock/test output visibly.

## Recovery and concurrency

The room runtime must be recoverable:

- `claimed` receipts need heartbeat/progress timestamps and stale recovery;
- adapter crashes finalize or recover receipts with auditable reasons;
- two adapters must not claim the same receipt unless the policy explicitly allows
  duplicate work;
- owner/reviewer/merge-gate states for code work should be visible so PR-first
  collaboration does not become accidental merge authority.

## Security and privacy boundaries

Open-room collaboration can easily leak private runtime data. Default policy
should be conservative:

- private pad/knowledge/skills are not displayed by default;
- absolute paths, secrets, API keys, raw tool args, and internal opaque IDs are
  redacted or hidden unless the user deliberately expands a trusted diagnostic
  view;
- transcript visibility is per-room and per-user, not global;
- attaching an agent to a room requires explicit authorization;
- lifecycle operations, if exposed at all, require the same confirmation and
  authority gates as the TUI/runtime;
- exported room transcripts should label mock/real participants and include
  redaction metadata.

## Relationship to Portal

Portal already visualizes live/replay network topology and status. Xingtan should
not duplicate that blindly.

Potential integration paths:

1. **Separate app/package:** fastest to iterate on room collaboration while
   linking to Portal for topology/history.
2. **Portal module:** adds room and receipt views alongside existing topology and
   replay APIs.
3. **Portal successor:** only appropriate if the room model subsumes topology,
   replay, and collaboration without losing the current portal's focused network
   observability.

The first product decision is which path to take. Until then, this contract should
be treated as a candidate architecture rather than a committed repository layout.

## Roadmap checklist

### Phase 0 — product / architecture review

- [ ] Decide repo/package placement: `Lingtai-AI/lingtai`, existing Portal, or a
      separate Xingtan package.
- [ ] Decide adapter route for LingTai agents: internal email, LICC, HTTP API, or
      new room protocol.
- [ ] Define permission model for room membership, transcript visibility, and
      lifecycle operations.
- [ ] Decide how room receipts map to existing runtime logs and portal history.

### Phase 1 — Web Frontend MVP

- [ ] Room list and room detail view.
- [ ] Participant registry and mock/real labels.
- [ ] Human room message compose.
- [ ] Receipt/status timeline.
- [ ] LingTai agent adapter that routes through an existing authorized runtime
      path.
- [ ] Mock/real adapter distinction in UI and logs.
- [ ] Downloadable debug/traces bundle with redaction metadata.

### Phase 2 — multi-agent collaboration workbench

- [ ] `@attention`, handoff, ask-all, and explicit reply-target routing.
- [ ] Task/PR/issue cards bound to rooms.
- [ ] Agent activity/stuck/blocked visualization.
- [ ] Owner/reviewer/merge-gate status.
- [ ] Collaboration checklist templates for code/release work.

### Phase 3 — deeper LingTai integration

- [ ] Safe summaries of pad/knowledge/skills with privacy gates.
- [ ] Avatar/daemon visualization entry points.
- [ ] Portal topology/history integration.
- [ ] Project recipe / standing rules / check-in cadence configuration.
- [ ] Room transcript distillation into knowledge, skills, or session journals.

## Acceptance criteria for future implementation PRs

A PR that claims to implement part of #167 should state which phase/checklist
item it covers and include evidence for that slice. For a first code PR, the
minimum credible slice is:

- one room can be created and displayed;
- participants are registered with mock/real/harness labels;
- a human message creates receipts for enabled participants;
- at least a mock or LingTai adapter claims, finalizes, and records a reply;
- receipt states are visible in the UI;
- disabled participants produce `blocked` receipts;
- stale/failed receipts are recoverable or at least visibly failed;
- private runtime data is not exposed by default;
- tests cover routing, receipt state transitions, and mock/real labeling.

## Non-goals for this design note

- It does not choose the final repository/package location.
- It does not replace the TUI or Portal.
- It does not implement room runtime, database schema, React UI, or adapters.
- It does not grant Web UI lifecycle authority over agents.
- It does not close Lingtai-AI/lingtai#167 by itself.

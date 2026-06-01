---
name: daily-reflection
description: >
  Daily self-examination across agents: scan logs and health, find errors or
  anomalies, summarize lessons learned, track progress, and generate an
  improvement report. Router for data collection, analysis/reporting, issue
  filing, worked examples, and SQLite-vs-JSONL log routing.
version: 1.1.0
---

# Daily Reflection — Router

> *"The unexamined agent is not worth running."*

This skill routes daily self-examination across a LingTai project. Use it at the
end of a work day, the start of a new day, after an incident, or for periodic
network health checks.

It scans agent state, logs, token usage, health indicators, errors, anomalies,
and lessons, then produces a dated report with action items. For deep event-trace
analysis, prefer the kernel SQLite sidecar route when available; JSONL shell
recipes remain useful for older projects and quick probes.

## When to use

- End of a work day — review what happened and catch problems before they compound.
- Start of a new day — understand network state before planning work.
- After a major incident — audit all agents for collateral damage.
- Weekly or periodic health checks — track trends and recurring issues.

## Reflection loop

```text
1. Discover all agents
2. For each agent: collect logs/events/token/health evidence
3. Aggregate findings across agents
4. Classify by severity
5. Extract lessons and patterns
6. Generate structured report
7. Optionally file GitHub issues for critical/high findings
8. Write report to a dated file
```

## Nested reference catalog

`daily-reflection` owns these nested references. They are parent-owned drill-down
files, not standalone top-level skills.

```yaml
- name: daily-reflection-data-collection
  location: reference/data-collection/SKILL.md
  description: |
    Discover agents, choose the time window, scan events/logs/token ledgers and
    health indicators, and route detailed event trace mining to SQLite/log.sqlite
    when available.
- name: daily-reflection-analysis-reporting
  location: reference/analysis-reporting/SKILL.md
  description: |
    Classify findings, extract lessons and patterns, generate the report
    template, optionally file GitHub issues, and write dated reports.
- name: daily-reflection-operations
  location: reference/operations/SKILL.md
  description: |
    Complete copy-paste checklist, anti-patterns, worked example, and
    customization options for daily or weekly reflection.
```

## Router table

| Need / keywords | Read |
|---|---|
| Find project `.lingtai/`; enumerate agents; choose UTC day window; scan `events.jsonl`, `agent.log`, `token_ledger.jsonl`, heartbeat/state/stamina; decide JSONL vs SQLite sidecar | `reference/data-collection/SKILL.md` |
| Severity classification; lessons/patterns; report template; optional GitHub issue filing; write report to disk | `reference/analysis-reporting/SKILL.md` |
| Full checklist; anti-patterns; worked example; customization for weekly trends or specific agents | `reference/operations/SKILL.md` |

## SQLite / trajectory-mining note

When a current `log.sqlite` sidecar is present, use the system manual's nested
SQLite reference for detailed event-trace metrics and anomaly mining:
`system-manual/reference/sqlite-log-query/SKILL.md`. Daily reflection should use
that output as one input to the broader daily report rather than duplicating all
SQL recipes here. The JSONL snippets in the data-collection reference remain
fallbacks for older projects and quick shell checks.

## Output contract

A daily reflection should produce a concise dated report with:

- network health summary;
- critical/high/medium/low findings with evidence;
- lessons learned and recurring patterns;
- discoveries and improvement candidates;
- action items for tomorrow;
- token usage summary when available;
- issue links only when explicitly authorized or already requested.

## Safety and side effects

- Reading logs and writing a local report is normal reflection work.
- Filing GitHub issues, closing issues, changing config, deleting logs, or
  scheduling recurring reflection jobs are external side effects; require
  explicit human authorization.
- Do not print secrets from logs. Redact tokens, API keys, bearer strings, and
  mailbox/private paths before sharing externally.
- Keep raw evidence local; summarize findings for humans.

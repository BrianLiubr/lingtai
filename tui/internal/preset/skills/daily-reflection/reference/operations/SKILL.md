---
name: daily-reflection-operations
description: >
  Nested daily-reflection reference for the complete copy-paste checklist,
  anti-patterns, worked example, and customization options for daily/weekly
  network reflection.
version: 1.0.0
---

# Daily Reflection Operations Reference

Nested daily-reflection reference. Open this for the full checklist, examples,
anti-patterns, and customization guidance.

## Anti-Patterns

- **Skimming logs** — Don't just grep for "error". Read context around errors to understand causation.
- **False positives** — A `capability_skipped` for an unused capability is noise, not a finding. Filter by what the agent actually needs.
- **Ignoring idle agents** — An agent that's been `asleep` for 3 days with a stale heartbeat might be stuck, not resting.
- **Reporting without action items** — Every finding above LOW should have a concrete next step.
- **Filing too many issues** — Only file GitHub issues for problems that need tracking beyond tomorrow. Most findings belong in the report only.
- **Ignoring successes** — The "Discoveries and Improvements" section is not optional. Agents learn. Capture what they learned.

---

## Worked Example

**Scenario:** You run daily-reflection at the end of May 10, 2026.

1. You find 38 agents under `.lingtai/`. 12 are active, 8 asleep, 18 idle/suspended.

2. Scanning `mimo-1/logs/events.jsonl`, you find:
   ```json
   {"type": "mcp_error", "ts": 1778571600.5, "agent_name": "mimo-1", "error": "Feishu config not found"}
   ```
   This is MEDIUM — Feishu MCP failed to load but the agent can function without it.

3. Scanning `telegram-patcher/logs/agent.log`, you find:
   ```
   2026-05-10 14:23:11,456 ERROR telegram: Message too long (4200 chars), truncated
   2026-05-10 14:23:11,457 ERROR telegram: Message too long (5100 chars), truncated
   2026-05-10 15:01:33,890 ERROR telegram: Message too long (4800 chars), truncated
   ```
   3 occurrences of the same error → promote from MEDIUM to HIGH. The agent repeatedly generates messages exceeding Telegram's 4096-char limit.

4. Scanning `xhelio-maintainer/logs/token_ledger.jsonl`, you find 340,000 input tokens with 0 cached tokens — pure cache misses. This agent's prompting strategy is inefficient. MEDIUM.

5. You notice `researcher-past-selves` has a heartbeat from 2 days ago but state is `active`. HIGH — the agent may be stuck.

6. You write the report to `reflections/reflection-2026-05-10.md` and update `pad.md`.

7. You file a GitHub issue for the stuck `researcher-past-selves` agent since it may need manual intervention (CPR or restart).

---

## Customization

Agents can customize this skill by adjusting thresholds:

- **Heartbeat staleness threshold:** Default 7200s (2h). Increase for agents with long sleep cycles.
- **Token usage alert threshold:** Default 5M input tokens/day. Adjust per agent workload.
- **API call alert threshold:** Default 200 calls/day. Adjust for high-throughput agents.
- **Issue filing:** Can be disabled entirely by skipping Step 7.
- **Report location:** Default `reflections/`. Can be changed to any directory.
- **Pad update:** Can be disabled if the agent manages its pad differently.

---
> **Found a bug or issue?** If you encounter any problems with this skill, load the `lingtai-issue-report` skill and follow its instructions to report it.

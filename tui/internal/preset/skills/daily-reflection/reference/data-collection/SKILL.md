---
name: daily-reflection-data-collection
description: >
  Nested daily-reflection reference for collecting reflection inputs: discovering
  agents, choosing the time window, scanning events/logs/token ledgers/health
  indicators, and preferring SQLite/log.sqlite trajectory analysis when available.
version: 1.0.0
---

# Daily Reflection Data Collection Reference

Nested daily-reflection reference. Open this when you need the concrete commands
for finding agents and collecting daily evidence.

> SQLite note: older projects may only have JSONL logs. When a current
> `log.sqlite` sidecar exists, prefer `system-manual/reference/sqlite-log-query`
> for detailed event-trace and trajectory/anomaly mining. If that reference is
> not bundled in the current agent, keep using the JSONL recipes here as
> compatibility fallbacks and quick shell probes.

## Step 1: Discover All Agents

Find the project root's `.lingtai/` directory and enumerate all agent directories.

```bash
# Find the project .lingtai/ directory
PROJECT_LINGTAI="$(git -C "$(pwd)" rev-parse --show-toplevel 2>/dev/null)/.lingtai"

# If not in a git repo, walk up from cwd
if [ ! -d "$PROJECT_LINGTAI" ]; then
  dir="$(pwd)"
  while [ "$dir" != "/" ]; do
    if [ -d "$dir/.lingtai" ]; then
      PROJECT_LINGTAI="$dir/.lingtai"
      break
    fi
    dir="$(dirname "$dir")"
  done
fi

# List all agent directories (those containing .agent.json)
for d in "$PROJECT_LINGTAI"/*/; do
  if [ -f "$d/.agent.json" ]; then
    echo "$d"
  fi
done
```

**Skip these directories** — they are not agents:
- `.git/`
- `.library_shared/`
- `.tui-asset/`
- `human/` (pseudo-agent, no logs)

**Identify each agent by reading** `.agent.json`:
- `agent_name` — the agent's name
- `state` — current state: `active`, `idle`, `asleep`, `suspended`
- `capabilities` — what tools the agent has

---

## Step 2: Determine the Time Window

The reflection covers a specific day. Default to today (UTC).

```bash
# Today's date boundaries (UTC)
TODAY=$(date -u +%Y-%m-%d)
DAY_START_EPOCH=$(date -u -j -f "%Y-%m-%d %H:%M:%S" "$TODAY 00:00:00" +%s 2>/dev/null || date -u -d "$TODAY 00:00:00" +%s)
DAY_END_EPOCH=$((DAY_START_EPOCH + 86400))
```

For `events.jsonl`, filter by the `ts` field (Unix epoch float).
For `agent.log`, filter by the timestamp prefix (`YYYY-MM-DD HH:MM:SS` or `HH:MM:SS`).

---

## Step 3: Scan Agent Logs

For each agent, scan two log sources.

### 3a. Parse `events.jsonl`

Location: `<agent-dir>/logs/events.jsonl`

Each line is a JSON object with at minimum: `type`, `agent_name`, `ts`.

**Error event types to look for:**

| Event Type Pattern | Severity | What It Means |
|---|---|---|
| `error`, `*_error`, `*_failed` | HIGH | Something broke |
| `capability_skipped` | MEDIUM | A capability failed to load |
| `heartbeat_timeout`, `heartbeat_failed` | HIGH | Agent may be unresponsive |
| `mail_bounce`, `mail_error` | MEDIUM | Communication failure |
| `mcp_error`, `mcp_*_failed` | MEDIUM | MCP tool integration broken |
| `daemon_error`, `daemon_crash` | HIGH | Background process died |
| `molt_*` | LOW | Session reset (track frequency) |
| `api_error`, `llm_error`, `glm_error` | HIGH | LLM/API call failed |
| `tool_error`, `tool_call_failed` | MEDIUM | A tool invocation failed |
| `refresh_complete` with missing capabilities | LOW | Capabilities didn't all load |

```bash
# Extract error/warning events for today from one agent
AGENT_DIR="<agent-dir>"
EVENTS="$AGENT_DIR/logs/events.jsonl"

if [ -f "$EVENTS" ]; then
  # Filter to today's events and find errors
  cat "$EVENTS" | while IFS= read -r line; do
    ts=$(echo "$line" | jq -r '.ts // empty')
    if [ -n "$ts" ]; then
      ts_int=${ts%.*}
      if [ "$ts_int" -ge "$DAY_START_EPOCH" ] && [ "$ts_int" -lt "$DAY_END_EPOCH" ]; then
        type=$(echo "$line" | jq -r '.type // empty')
        case "$type" in
          *error*|*failed*|*crash*|*bounce*|*timeout*|*skipped*)
            echo "$line"
            ;;
        esac
      fi
    fi
  done
fi
```

**Better approach — use `jq` directly:**

```bash
jq -c "select(.ts >= $DAY_START_EPOCH and .ts < $DAY_END_EPOCH) | select(.type | test(\"error|failed|crash|bounce|timeout|skipped\"))" "$EVENTS"
```

### 3b. Parse `agent.log`

Location: `<agent-dir>/logs/agent.log`

Format varies but common patterns:

```
HH:MM:SS LEVEL component: [agent] message
YYYY-MM-DD HH:MM:SS,mmm LEVEL component message
```

**Grep patterns for problems:**

```bash
AGENT_LOG="$AGENT_DIR/logs/agent.log"

if [ -f "$AGENT_LOG" ]; then
  # Filter today's entries and find errors/warnings
  grep -E "^$TODAY|ERROR|CRITICAL|WARNING|Traceback|Exception|failed|timed out" "$AGENT_LOG" | \
    grep -i -E "error|warning|critical|exception|failed|timeout|refused|denied|crash|abort|fatal"
fi
```

**Key patterns to match (regex):**

```
ERROR                          — any error-level log
CRITICAL                       — critical failures
WARNING.*(?:skip|fail|timeout) — warnings with actionable content
Traceback                      — Python tracebacks
Exception                      — Exception messages
refused|denied|unauthorized    — auth/permission failures
timed? out                     — timeouts
ECONNREFUSED|ECONNRESET        — network failures
429|rate.limit                 — rate limiting
OOM|out of memory              — resource exhaustion
```

### 3c. Check `token_ledger.jsonl`

Location: `<agent-dir>/logs/token_ledger.jsonl`

Look for abnormal token usage — signs of runaway loops or inefficiency.

```bash
TOKEN_LOG="$AGENT_DIR/logs/token_ledger.jsonl"

if [ -f "$TOKEN_LOG" ]; then
  # Sum today's token usage
  jq -s "[.[] | select(.ts >= \"${TODAY}T00:00:00Z\" and .ts < \"${TOMORROW}T00:00:00Z\")] | {
    total_input: (map(.input) | add // 0),
    total_output: (map(.output) | add // 0),
    total_thinking: (map(.thinking) | add // 0),
    api_calls: length,
    models: (map(.model) | unique)
  }" "$TOKEN_LOG"
fi
```

**Red flags:**
- `api_calls` > 200 in a day for a single agent
- `total_input` > 5,000,000 tokens (possible context loop)
- `total_output` > 500,000 tokens (possible generation loop)
- High `input` with near-zero `cached` (cache misses — inefficient prompting)

### 3d. Check Agent Health Indicators

```bash
# Check heartbeat freshness
HEARTBEAT="$AGENT_DIR/.agent.heartbeat"
if [ -f "$HEARTBEAT" ]; then
  HEARTBEAT_EPOCH=$(cat "$HEARTBEAT")
  NOW_EPOCH=$(date +%s)
  STALE_SECONDS=$((NOW_EPOCH - ${HEARTBEAT_EPOCH%.*}))
  if [ "$STALE_SECONDS" -gt 7200 ]; then
    echo "WARNING: Heartbeat stale by ${STALE_SECONDS}s"
  fi
fi

# Check agent state
STATE=$(jq -r '.state' "$AGENT_DIR/.agent.json")

# Check stamina
STAMINA_LEFT=$(jq -r '.runtime.stamina_left // empty' "$AGENT_DIR/.status.json" 2>/dev/null)
STAMINA_MAX=$(jq -r '.runtime.stamina // empty' "$AGENT_DIR/.status.json" 2>/dev/null)
```

### 3e. Mailbox / channel backlog

Reflection should catch stuck or bounced communication before it becomes an
operational surprise. Do a lightweight mailbox scan; quote only counts or
non-sensitive subjects in any shared report.

```bash
find .lingtai -type f -path '*/mailbox/inbox/*' | head -50
find .lingtai -type f -path '*/mailbox/outbox/*' | head -50
```

---

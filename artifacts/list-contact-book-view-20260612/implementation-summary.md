# `lingtai-tui list` decentralized contact-book view

## Changed behavior

- `lingtai-tui list [dir]` keeps the old process-list role but now adds a `ROLE` column (`MAIN`, `AGENT`, `HUMAN`, or `?`). `MAIN` is computed from the running agent's local `.agent.json` admin map, not from a central address book.
- `lingtai-tui list --detailed [dir]` adds public identity/state details: state, address, agent name, nickname, project path, and agent directory, and public IM handles from each agent-owned `system/mcp_identities/*.json` identity file.
- `lingtai-tui list --admin [dir]` implies detailed mode and shows the admin map summary used as the main-agent判定依据.
- No `~/.lingtai-tui/addressbook.jsonl` or other centralized contact database is created.

## Files changed

- `tui/list_common.go`, `tui/list_unix.go`, `tui/list_windows.go`: shared parsing/rendering/metadata helpers plus platform process discovery.
- `tui/list_common_test.go`: focused tests for flag parsing, role/admin summaries, and table rendering.
- `tui/main.go`: updated help usage.
- `README.md`, `README.zh.md`: updated shell examples.
- `tui/internal/preset/skills/lingtai-tutorial-guide/reference/communication/SKILL.md`: documents decentralized IM-handle ownership.
- `tui/ANATOMY.md`: records the new list helper surface.

## Validation

- `cd tui && go test ./...` passed before the final path-column cleanup.
- After cleanup: `cd tui && go test . ./internal/tui ./internal/preset -count=1` passed.
- Windows compile check: `cd tui && GOOS=windows go test -c -o /tmp/lingtai-tui-windows-test.exe .` passed.
- Smoke: `cd tui && go run . list --detailed /Users/huangzesen/work/projects/lingtai-dev/dev-1` showed `MAIN` for `mimo-1` and `AGENT` for `file-search-scout`.
- `git diff --check` passed.

## Caveat

A first Claude Code implementation run produced no file changes for several minutes and was cancelled; implementation was completed manually to unblock Jason's waiting task.

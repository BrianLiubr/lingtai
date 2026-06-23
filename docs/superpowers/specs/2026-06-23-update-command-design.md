# `/update` command — kernel-only, confirm-then-update (PR 1 of 2)

**Date:** 2026-06-23
**Status:** Design approved
**Repo:** `lingtai` (TUI, Go)

## Context

Today the TUI has no dedicated "update" command. All updating happens implicitly inside `/doctor`, whose forced-update phase (`runDoctor` → `config.RunDoctorUpdate`) mutates **five** surfaces before any diagnostics run:

1. TUI binary self-upgrade via Homebrew (`checkTUI`)
2. Python `lingtai` kernel via uv/pip (`checkPythonRuntime` → `UpgradePythonRuntime`)
3. File-search native deps (`checkFileSearchNative`)
4. Preset bootstrap/migration (`preset.Bootstrap`) — this is what triggers the kernel-side `version:2` preset migration that has renamed saved presets out from under running agents
5. Library + commands refresh (`PopulateBundledLibrary`, `ExportCommandsJSON`)

This bundling makes `/doctor` a "doctor that heals" — surprising, and the cause of saved-preset loss. The fix is split across two PRs:

- **PR 1 (this spec):** Add a focused `/update` command that updates the **kernel only**, gated behind an explicit confirmation. Touches nothing in `/doctor`.
- **PR 2 (later):** Make `/doctor` diagnostic-only and add a gated heal action that reuses PR 1's kernel-update path. Spec'd separately.

## Goal

A new interactive `/update` command that:

- Checks the installed Python `lingtai` kernel version against the latest PyPI release (read-only).
- Shows `current → latest` and asks the user to confirm before mutating anything.
- On confirm, runs **only** the kernel update (uv/pip), preserving the existing dev-editable safety (never clobbers an editable local checkout).
- Updates **nothing else** — no brew TUI upgrade, no preset Bootstrap, no file-search, no library/commands rewrite.

Non-goals (PR 1):

- No changes to interactive `/doctor` or the headless CLI `lingtai-tui doctor`.
- No changes to the kernel-side `version:2` preset migration (separate repo).
- No TUI binary self-upgrade in `/update`.

## Behavior

`/update` is a dedicated view (`UpdateModel`) modeled on the existing `DoctorModel`. State machine:

```
stateChecking ──> stateConfirm ──(confirm)──> stateUpdating ──> stateDone
                      │                                            ▲
                      └──(already current / editable dev)──────────┘
                      └──(cancel)──> back to mail view
```

- **stateChecking** (read-only): resolve the managed venv (`~/.lingtai-tui/runtime/venv`), read installed kernel version, fetch latest PyPI version. Detect editable/dev installs. No mutation, no network writes.
- **stateConfirm**: if an update is available, render `current → latest` plus an `Update now / Cancel` selector using the existing index-selector idiom (`swapConfirmIdx`-style, arrow keys + enter). If already up to date, or the install is an editable dev checkout, skip directly to **stateDone** with an informational message ("already current" / "editable dev install, skipping").
- **stateUpdating**: run `config.RunKernelUpdate(globalDir, force=true)` — kernel only. Stream result lines.
- **stateDone**: show outcome; `esc` returns to mail view.

The confirmation is mandatory before any install command runs — `/update` never mutates on a single keystroke.

## Architecture

### `internal/config/venv.go`

Add two functions; refactor the kernel check so inspect and apply share logic.

- `InspectKernel(globalDir string) KernelStatus` — **read-only**. Returns:
  ```go
  type KernelStatus struct {
      Installed   string   // installed lingtai version, or "" if unimportable
      Latest      string   // latest PyPI version, or "" if lookup failed
      Editable    bool     // editable/dev install detected
      NeedsUpdate bool     // false when editable, missing-latest, or installed==latest
      Lines       []DoctorLine
  }
  ```
  Reuses existing helpers (`VenvPython`, `pythonLingtaiVersion`, `isEditableLingtaiInstall`, `fetchLatestPyPIVersion`). Issues **no** install/brew commands.

- `RunKernelUpdate(globalDir string, force bool) DoctorReport` — runs **only** the kernel path (equivalent to today's `checkPythonRuntime` → `UpgradePythonRuntime`). Does **not** call `checkTUI` or `checkFileSearchNative`. Existing dev-editable gates in `UpgradePythonRuntime` are preserved unchanged.

`RunDoctorUpdate` is left intact for the existing callers (`internal/tui/doctor.go`, `main.go:doctorMain`) so PR 1 changes no existing behavior.

### `internal/tui/update.go` (new)

`UpdateModel` with `Init/Update/View`, following `DoctorModel` conventions (async work via `tea.Cmd` returning a result msg; `esc` → `ViewChangeMsg{View: "mail"}`). Holds: `orchDir`, `globalDir`, `state`, `status KernelStatus`, `confirmIdx`, `resultLines`, sizing.

### `internal/tui/app.go`

- Add `appViewUpdate` to the view enum and an `update UpdateModel` field.
- Add dispatch `case "update":` mirroring `case "doctor":` — set `currentView`, construct `NewUpdateModel(targetDir, globalDir)`, batch `Init()` + `sendSize()`.
- Route window/key/result msgs to `a.update` and add `content = a.update.View()` to the render switch.

### `internal/tui/palette.go`

Register `{Name: "update", Description: "palette.update", Detail: "cmd.update"}` in `DefaultCommands()`, placed near `doctor`.

### i18n (`i18n/en.json`, `i18n/zh.json`, `i18n/wen.json`)

Add keys:
- `palette.update`, `cmd.update`
- `update.title`, `update.checking`
- `update.current_latest` (formatted: current → latest)
- `update.up_to_date`, `update.editable_skip`
- `update.confirm`, `update.cancel`, `update.prompt`
- `update.updating`, `update.done`, `update.failed`

## Error handling

- Venv missing/unimportable: `InspectKernel` reports it; `stateConfirm` offers to run the update anyway (which `EnsureVenv`-rebuilds via the existing `UpgradePythonRuntime` path) — same recovery the current doctor offers, just gated.
- PyPI lookup failure: report as a warning; if `force` confirmed, still attempt upgrade (matches existing `UpgradePythonRuntime` semantics).
- Install command failure: surfaced in `resultLines` as a FAIL; `stateDone` shows failure, `esc` exits.
- Editable dev install: never reinstalled; reported and skipped.

## Testing

- `venv_test.go`:
  - `InspectKernel` issues no `brew`/`pip`/`uv install` commands (assert via test `CommandRunner`); returns `NeedsUpdate=false` when `installed==latest`, and `Editable=true` → `NeedsUpdate=false` for an editable install.
  - `RunKernelUpdate` runs exactly one `uv/pip install --upgrade lingtai` for a non-editable out-of-date install, and zero for an editable one.
- `update_test.go`:
  - Healthy/up-to-date → model goes `stateChecking` → `stateDone` with no confirm prompt.
  - Out-of-date → `stateConfirm` appears; selecting "Cancel" issues no install; selecting "Update now" transitions to `stateUpdating` and calls `RunKernelUpdate`.

## Rollout

PR 1 is additive: a new command and two new `config` functions. No existing command changes behavior. PR 2 then refactors `/doctor` to diagnostic-only and reuses `RunKernelUpdate` inside its gated heal.

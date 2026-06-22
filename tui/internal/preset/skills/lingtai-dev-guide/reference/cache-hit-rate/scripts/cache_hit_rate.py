#!/usr/bin/env python3
"""Recent prompt-cache hit rate from LingTai token ledgers (read-only).

Cache hit rate is computed per time window as:

    hit_rate = sum(cached) / sum(input)

over all ledger entries whose ``ts`` falls within the window. Both fields come
straight from ``logs/token_ledger.jsonl`` (see
``lingtai_kernel/token_ledger.py``):

    input   total prompt/input tokens for the call (already INCLUDES the
            cached portion; for the Anthropic adapter this is
            raw_input + cache_read + cache_write)
    cached  cache-read input tokens served from the provider prompt cache
            (a subset of ``input``; cache *writes* are billed under input but
            are NOT counted as cached by the native adapters)

Because ``cached`` is a subset of ``input``, the ratio is bounded in [0, 1].
The metric is provider-agnostic: the kernel normalizes every provider's usage
into the same ``input``/``cached`` fields before the entry is written.

Default windows (lookback from "now", UTC): 1h, 5h, 1d, 3d.

This script is strictly READ-ONLY: it opens ledger files for reading and never
writes, mutates, or touches runtime state. Standard library only.
"""
from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

LEDGER_NAME = "token_ledger.jsonl"

# (label, timedelta). Order is preserved in output.
DEFAULT_WINDOWS: list[tuple[str, timedelta]] = [
    ("1h", timedelta(hours=1)),
    ("5h", timedelta(hours=5)),
    ("1d", timedelta(days=1)),
    ("3d", timedelta(days=3)),
]

_WINDOW_UNITS = {
    "s": timedelta(seconds=1),
    "m": timedelta(minutes=1),
    "h": timedelta(hours=1),
    "d": timedelta(days=1),
    "w": timedelta(weeks=1),
}


def parse_window(token: str) -> tuple[str, timedelta]:
    """Parse a window spec like ``1h``, ``5h``, ``1d``, ``3d``, ``90m``."""
    token = token.strip().lower()
    if len(token) < 2 or token[-1] not in _WINDOW_UNITS:
        raise argparse.ArgumentTypeError(
            f"bad window {token!r}: expected <int><s|m|h|d|w>, e.g. 1h 5h 1d 3d"
        )
    try:
        n = int(token[:-1])
    except ValueError:
        raise argparse.ArgumentTypeError(
            f"bad window {token!r}: expected <int><s|m|h|d|w>, e.g. 1h 5h 1d 3d"
        )
    if n <= 0:
        raise argparse.ArgumentTypeError(f"window must be positive: {token!r}")
    return token, _WINDOW_UNITS[token[-1]] * n


def parse_ts(value: str) -> datetime | None:
    """Parse a ledger timestamp into an aware UTC datetime, or None if invalid.

    Ledger timestamps are written as ``%Y-%m-%dT%H:%M:%SZ`` (UTC). We also
    tolerate an explicit ``+00:00`` offset for forward compatibility.
    """
    if not isinstance(value, str) or not value:
        return None
    v = value.strip()
    try:
        if v.endswith("Z"):
            v = v[:-1] + "+00:00"
        dt = datetime.fromisoformat(v)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt.astimezone(timezone.utc)


def resolve_ledgers(path: Path) -> tuple[list[Path], str]:
    """Resolve an input path to the ledger file(s) to read.

    Returns ``(ledger_paths, mode)`` where mode is one of ``file``,
    ``workdir``, ``root``.

    Resolution rules (chosen to avoid double-counting daemon usage):

    * A file path -> that single ledger.
    * A directory containing ``logs/token_ledger.jsonl`` -> that one agent/run
      ledger only. We deliberately do NOT recurse into ``daemons/*/logs`` here,
      because daemon calls are ALSO tagged into the parent agent ledger
      (source/em_id/run_id), so the agent ledger already includes them. Reading
      both would double-count.
    * Any other directory (a project root, e.g. ``.lingtai/``) -> every direct
      child ``<child>/logs/token_ledger.jsonl``. Nested ``daemons/`` ledgers are
      excluded for the same double-count reason.
    """
    if path.is_file():
        return [path], "file"
    if not path.is_dir():
        return [], "missing"

    own = path / "logs" / LEDGER_NAME
    if own.is_file():
        return [own], "workdir"

    found: list[Path] = []
    for child in sorted(path.iterdir()):
        if not child.is_dir():
            continue
        if child.name == "daemons":
            continue
        candidate = child / "logs" / LEDGER_NAME
        if candidate.is_file():
            found.append(candidate)
    return found, "root"


def scan_ledger(ledger: Path, source: str | None) -> dict:
    """Read one ledger, returning entries plus skip diagnostics."""
    entries: list[tuple[datetime, int, int]] = []
    bad_json = 0
    bad_ts = 0
    bad_fields = 0
    source_filtered = 0
    total_lines = 0
    try:
        text = ledger.read_text(encoding="utf-8")
    except OSError as exc:
        return {"error": str(exc), "entries": entries}
    for line in text.splitlines():
        line = line.strip()
        if not line:
            continue
        total_lines += 1
        try:
            d = json.loads(line)
        except json.JSONDecodeError:
            bad_json += 1
            continue
        if source is not None and d.get("source") != source:
            source_filtered += 1
            continue
        dt = parse_ts(d.get("ts"))
        if dt is None:
            bad_ts += 1
            continue
        inp = d.get("input")
        cac = d.get("cached")
        if not isinstance(inp, (int, float)) or not isinstance(cac, (int, float)):
            bad_fields += 1
            continue
        entries.append((dt, int(inp), int(cac)))
    return {
        "error": None,
        "entries": entries,
        "total_lines": total_lines,
        "bad_json": bad_json,
        "bad_ts": bad_ts,
        "bad_fields": bad_fields,
        "source_filtered": source_filtered,
    }


def compute_windows(
    entries: list[tuple[datetime, int, int]],
    now: datetime,
    windows: list[tuple[str, timedelta]],
) -> list[dict]:
    """Compute per-window cache-hit aggregates."""
    out = []
    for label, delta in windows:
        cutoff = now - delta
        calls = 0
        input_sum = 0
        cached_sum = 0
        for dt, inp, cac in entries:
            if dt >= cutoff and dt <= now:
                calls += 1
                input_sum += inp
                cached_sum += cac
        rate = (cached_sum / input_sum) if input_sum > 0 else None
        out.append({
            "window": label,
            "cutoff": cutoff.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "calls": calls,
            "input": input_sum,
            "cached": cached_sum,
            "hit_rate": rate,
        })
    return out


def fmt_rate(rate) -> str:
    return "   n/a" if rate is None else f"{rate * 100:5.1f}%"


def render_text(report: dict) -> str:
    lines: list[str] = []
    lines.append(f"Cache hit rate  (now={report['now']}, mode={report['mode']})")
    src = report.get("source")
    if src:
        lines.append(f"source filter: {src}")
    lines.append(f"ledgers read: {len(report['ledgers'])}")
    for lp in report["ledgers"]:
        lines.append(f"  - {lp}")
    lines.append("")
    header = f"{'window':>7}  {'calls':>7}  {'input':>14}  {'cached':>14}  {'hit_rate':>8}"
    lines.append(header)
    lines.append("-" * len(header))
    for w in report["windows"]:
        lines.append(
            f"{w['window']:>7}  {w['calls']:>7}  {w['input']:>14,}  "
            f"{w['cached']:>14,}  {fmt_rate(w['hit_rate']):>8}"
        )
    skips = report["skipped"]
    noted = {k: v for k, v in skips.items() if v}
    if noted:
        lines.append("")
        lines.append("skipped lines: " + ", ".join(f"{k}={v}" for k, v in noted.items()))
    if report["empty_overall"]:
        lines.append("")
        lines.append("NOTE: no valid ledger entries found in any window.")
    return "\n".join(lines)


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="cache_hit_rate.py",
        description="Recent prompt-cache hit rate from LingTai token ledgers "
                    "(read-only).",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Examples:\n"
            "  # current agent workdir (e.g. .lingtai/codex)\n"
            "  cache_hit_rate.py .\n\n"
            "  # a specific agent workdir\n"
            "  cache_hit_rate.py /path/.lingtai/codex\n\n"
            "  # a whole project root: per-agent ledgers aggregated\n"
            "  cache_hit_rate.py /path/.lingtai\n\n"
            "  # a single ledger file, custom windows, JSON out\n"
            "  cache_hit_rate.py logs/token_ledger.jsonl --windows 1h 6h 1d --json\n\n"
            "  # only main-chat turns, deterministic clock for testing\n"
            "  cache_hit_rate.py . --source main --now 2026-06-22T01:00:00Z\n"
        ),
    )
    p.add_argument(
        "path",
        nargs="?",
        default=".",
        help="Agent workdir, project root, or a token_ledger.jsonl file "
             "(default: current directory).",
    )
    p.add_argument(
        "--windows",
        nargs="+",
        type=parse_window,
        default=None,
        metavar="W",
        help="Lookback windows, e.g. 1h 5h 1d 3d (default), 90m, 1w.",
    )
    p.add_argument(
        "--source",
        default=None,
        help="Only count entries with this source tag "
             "(e.g. main, soul, tc_wake, daemon).",
    )
    p.add_argument(
        "--now",
        default=None,
        metavar="ISO",
        help="Override 'now' (UTC ISO8601, e.g. 2026-06-22T01:00:00Z) for "
             "deterministic/testing runs. Defaults to the current UTC time.",
    )
    p.add_argument(
        "--json",
        action="store_true",
        help="Emit a JSON report instead of the text table.",
    )
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)

    if args.now is not None:
        now = parse_ts(args.now)
        if now is None:
            print(f"error: bad --now value {args.now!r} (want UTC ISO8601 "
                  f"like 2026-06-22T01:00:00Z)", file=sys.stderr)
            return 2
    else:
        now = datetime.now(timezone.utc)

    windows = args.windows if args.windows else DEFAULT_WINDOWS

    path = Path(args.path).expanduser()
    ledgers, mode = resolve_ledgers(path)
    if mode == "missing":
        print(f"error: path not found: {path}", file=sys.stderr)
        return 2
    if not ledgers:
        print(f"error: no {LEDGER_NAME} found under {path} "
              f"(looked for <path>/logs/{LEDGER_NAME} and "
              f"<path>/*/logs/{LEDGER_NAME})", file=sys.stderr)
        return 1

    all_entries: list[tuple[datetime, int, int]] = []
    skipped = {"bad_json": 0, "bad_ts": 0, "bad_fields": 0, "source_filtered": 0}
    read_paths: list[str] = []
    for ledger in ledgers:
        res = scan_ledger(ledger, args.source)
        if res.get("error"):
            print(f"warning: could not read {ledger}: {res['error']}",
                  file=sys.stderr)
            continue
        read_paths.append(str(ledger))
        all_entries.extend(res["entries"])
        for k in skipped:
            skipped[k] += res.get(k, 0)

    win_results = compute_windows(all_entries, now, windows)
    report = {
        "now": now.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "mode": mode,
        "source": args.source,
        "ledgers": read_paths,
        "windows": win_results,
        "skipped": skipped,
        "empty_overall": len(all_entries) == 0,
    }

    if args.json:
        print(json.dumps(report, indent=2))
    else:
        print(render_text(report))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

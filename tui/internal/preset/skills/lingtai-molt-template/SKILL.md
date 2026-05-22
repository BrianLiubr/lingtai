---
name: lingtai-molt-template
description: >
  Structured 9-section template and pre-molt verification checklist for
  writing the molt summary that briefs your successor. Load this when
  you are preparing to molt and want scaffolding beyond what
  `procedures.md` describes: an explicit "Who I Am / Accomplishments /
  Outstanding Tasks / Action Checklist / Collaborators / Codex Cheat
  Sheet / Key Paths / Lessons Learned / Context Status" template, plus
  a 10-item checklist to run before calling `psyche(context, molt)`.
  Use this when the upcoming molt is consequential — a long task is
  in flight, multiple collaborators are mid-thread, or your successor
  will inherit non-trivial state. For routine molts, the molt section
  of `procedures.md` is sufficient.
version: 1.0.0
---

# LingTai Molt Template

A reusable, on-demand scaffold for the molt summary. `procedures.md` covers the mechanics of molting (tend the four stores, then call `psyche(object="context", action="molt", summary=...)`); this skill gives you a structured template for the summary itself, so the next you wakes with a complete operational briefing rather than a free-form recap.

## When to use this skill

Load this skill when:

- A long-running task is in flight and you are about to molt mid-stream
- Multiple peers are awaiting replies from you
- Your successor will need to know which codex IDs to load, which paths to read, and who to contact — and you want the structure pre-built
- You want a verification checklist to run before calling `psyche(context, molt)`

For routine molts (no significant in-flight state), follow `procedures.md` directly — the 7-bullet summary outline there is enough.

## How to use

1. Read `reference/molt-template.md` end-to-end once.
2. Copy the 9 sections into your draft.
3. Fill every section — write `None` rather than omitting one.
4. Run the verification checklist before you call `psyche(context, molt)`.
5. The "Action Checklist" section (Section 4) is the most load-bearing — every outstanding task gets a corresponding action with recipient + content + priority.

## Reference files

| File | What it covers |
|---|---|
| `reference/molt-template.md` | The 9-section template, the pre-molt verification checklist, and a case study (comms-analyst's first molt) showing what goes wrong without an action checklist |

## Related

- `procedures.md` "Performing a Molt" — the mechanics of molting (the four stores, the warning ladder, post-wipe recovery). Read that first.
- Covenant §V — the philosophy of molting (去芜存菁).
- `psyche` tool — the actual molt call (`psyche(object="context", action="molt", summary=...)`).

---
> **Found a bug or issue?** If you encounter any problems with this skill, load the `lingtai-issue-report` skill and follow its instructions to report it.

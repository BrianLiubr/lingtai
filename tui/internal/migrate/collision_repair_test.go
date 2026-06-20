package migrate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCollision_From37_GetsBothRepairs proves a project at v37 (origin/main state
// before any collision PR) receives both m038 (skills-paths) and m039
// (context-preset repair) and ends at CurrentVersion=39.
func TestCollision_From37_GetsBothRepairs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	lingtaiDir := filepath.Join(tmp, ".lingtai")
	if err := os.MkdirAll(lingtaiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Seed a fake replacement preset so the preset repair can fire.
	savedDir := filepath.Join(tmp, ".lingtai-tui", "presets", "saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "codex-gpt5.5.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Agent that needs BOTH repairs:
	//   - skills.paths missing from capabilities (m038 target)
	//   - root context_limit without llm.context_limit (m039 target)
	//   - stale codex.json preset pointer (m039 target)
	writeAgentInit(t, lingtaiDir, "agent1", `{
  "manifest": {
    "context_limit": 300000,
    "llm": {"provider": "x", "model": "y"},
    "capabilities": {"skills": {"library_limit": 10}},
    "preset": {
      "active": "~/.lingtai-tui/presets/saved/codex.json",
      "default": "~/.lingtai-tui/presets/saved/codex.json",
      "allowed": ["~/.lingtai-tui/presets/saved/codex.json"]
    }
  }
}`)

	// Start at v37.
	writeMeta(t, lingtaiDir, 37)

	if err := Run(lingtaiDir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Version must reach 39.
	meta := readMeta(t, lingtaiDir)
	if meta.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", meta.Version, CurrentVersion)
	}

	initPath := filepath.Join(lingtaiDir, "agent1", "init.json")
	// m038: skills.paths should have been added.
	skills := readAgentSkills(t, initPath)
	if !reflect.DeepEqual(skills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("skills.paths after m038 = %#v, want %#v", skills["paths"], defaultPresetSkillsPaths)
	}
	// m039: llm.context_limit should have been set from root.
	manifest := readManifest(t, initPath)
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit after m039 = %#v, want 300000", got)
	}
	// m039: stale codex preset should have been rewritten.
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("preset.active after m039 = %#v, want %q", got, want)
	}
}

// TestCollision_AlreadyAt38_SkillsPathsBranch simulates a project stamped at
// v38 by the PR #340 binary (which ran migrateAgentInitSkillsPaths but not
// the context/preset repair). Running the new binary should run only m039.
func TestCollision_AlreadyAt38_SkillsPathsBranch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	lingtaiDir := filepath.Join(tmp, ".lingtai")
	if err := os.MkdirAll(lingtaiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	savedDir := filepath.Join(tmp, ".lingtai-tui", "presets", "saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "codex-gpt5.5.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Agent already received m038 (skills.paths present) but still needs m039.
	writeAgentInit(t, lingtaiDir, "agent1", `{
  "manifest": {
    "context_limit": 300000,
    "llm": {"provider": "x", "model": "y"},
    "capabilities": {
      "skills": {"paths": ["../.library_shared", "~/.lingtai-tui/utilities"]}
    },
    "preset": {
      "active": "~/.lingtai-tui/presets/saved/codex.json",
      "default": "~/.lingtai-tui/presets/saved/codex.json",
      "allowed": ["~/.lingtai-tui/presets/saved/codex.json"]
    }
  }
}`)

	// Simulate PR #340 binary: project is at v38, skills-paths already applied.
	writeMeta(t, lingtaiDir, 38)

	if err := Run(lingtaiDir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := readMeta(t, lingtaiDir)
	if meta.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", meta.Version, CurrentVersion)
	}

	initPath := filepath.Join(lingtaiDir, "agent1", "init.json")
	manifest := readManifest(t, initPath)

	// m039: llm.context_limit must have been set.
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000", got)
	}
	// m039: stale preset rewritten.
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("preset.active = %#v, want %q", got, want)
	}
	// m038 result (skills.paths) must still be intact.
	skills := readAgentSkills(t, initPath)
	wantPaths := []interface{}{"../.library_shared", "~/.lingtai-tui/utilities"}
	if !reflect.DeepEqual(skills["paths"], wantPaths) {
		t.Fatalf("skills.paths damaged: %#v", skills["paths"])
	}
}

// TestCollision_AlreadyAt38_ContextPresetBranch simulates a project stamped at
// v38 by the PR #357 binary (which ran migrateAgentInitContextPresetRepair but
// not skills-paths). Running the new binary should run only m038 (then m039,
// which is idempotent since the preset repair already ran).
func TestCollision_AlreadyAt38_ContextPresetBranch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	lingtaiDir := filepath.Join(tmp, ".lingtai")
	if err := os.MkdirAll(lingtaiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	savedDir := filepath.Join(tmp, ".lingtai-tui", "presets", "saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(savedDir, "codex-gpt5.5.json")
	if err := os.WriteFile(replacement, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Agent already received PR #357 repair: llm.context_limit set, preset
	// points at replacement. Still needs m038 (skills.paths missing).
	writeAgentInit(t, lingtaiDir, "agent1", `{
  "manifest": {
    "context_limit": 300000,
    "llm": {"provider": "x", "model": "y", "context_limit": 300000},
    "capabilities": {"bash": {"yolo": true}},
    "preset": {
      "active": "~/.lingtai-tui/presets/saved/codex-gpt5.5.json",
      "default": "~/.lingtai-tui/presets/saved/codex-gpt5.5.json",
      "allowed": ["~/.lingtai-tui/presets/saved/codex-gpt5.5.json"]
    }
  }
}`)

	// Simulate PR #357 binary: project is at v38, context/preset already applied.
	writeMeta(t, lingtaiDir, 38)

	if err := Run(lingtaiDir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := readMeta(t, lingtaiDir)
	if meta.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", meta.Version, CurrentVersion)
	}

	initPath := filepath.Join(lingtaiDir, "agent1", "init.json")

	// m038: skills.paths must have been added by m038.
	skills := readAgentSkills(t, initPath)
	if !reflect.DeepEqual(skills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("skills.paths = %#v, want %#v", skills["paths"], defaultPresetSkillsPaths)
	}

	// m039 must not have damaged the already-repaired fields.
	manifest := readManifest(t, initPath)
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000 (must not be zeroed)", got)
	}
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("preset.active = %#v, want %q (already repaired, must not change)", got, want)
	}
}

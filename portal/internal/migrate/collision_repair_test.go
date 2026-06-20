package migrate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestPortalCollision_From37_GetsBothRepairs proves a project at v37 receives
// both m038 (skills-paths) and m039 (context-preset repair) and ends at 39.
func TestPortalCollision_From37_GetsBothRepairs(t *testing.T) {
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

	writeAgentInitPortal(t, lingtaiDir, "agent1", `{
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

	writeMeta(t, lingtaiDir, 37)

	if err := Run(lingtaiDir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := readMeta(t, lingtaiDir)
	if meta.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", meta.Version, CurrentVersion)
	}

	initPath := filepath.Join(lingtaiDir, "agent1", "init.json")
	skills := readAgentSkillsPortal(t, initPath)
	if !reflect.DeepEqual(skills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("skills.paths = %#v, want %#v", skills["paths"], defaultPresetSkillsPaths)
	}
	manifest := readManifestPortal(t, initPath)
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000", got)
	}
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("preset.active = %#v, want %q", got, want)
	}
}

// TestPortalCollision_AlreadyAt38_SkillsPathsBranch simulates a project stamped
// at v38 by the PR #340 binary; the new binary must apply m039 (which includes
// both skills-paths idempotent + context-preset repair).
func TestPortalCollision_AlreadyAt38_SkillsPathsBranch(t *testing.T) {
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

	writeAgentInitPortal(t, lingtaiDir, "agent1", `{
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

	writeMeta(t, lingtaiDir, 38)

	if err := Run(lingtaiDir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := readMeta(t, lingtaiDir)
	if meta.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", meta.Version, CurrentVersion)
	}

	initPath := filepath.Join(lingtaiDir, "agent1", "init.json")
	manifest := readManifestPortal(t, initPath)

	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000", got)
	}
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("preset.active = %#v, want %q", got, want)
	}
	skills := readAgentSkillsPortal(t, initPath)
	wantPaths := []interface{}{"../.library_shared", "~/.lingtai-tui/utilities"}
	if !reflect.DeepEqual(skills["paths"], wantPaths) {
		t.Fatalf("skills.paths damaged: %#v", skills["paths"])
	}
}

// TestPortalCollision_AlreadyAt38_ContextPresetBranch simulates a project stamped
// at v38 by the PR #357 binary; the new binary must apply m039 (which calls
// skills-paths idempotent first, then no-ops the already-done preset repair).
func TestPortalCollision_AlreadyAt38_ContextPresetBranch(t *testing.T) {
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

	writeAgentInitPortal(t, lingtaiDir, "agent1", `{
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

	writeMeta(t, lingtaiDir, 38)

	if err := Run(lingtaiDir); err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := readMeta(t, lingtaiDir)
	if meta.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", meta.Version, CurrentVersion)
	}

	initPath := filepath.Join(lingtaiDir, "agent1", "init.json")

	skills := readAgentSkillsPortal(t, initPath)
	if !reflect.DeepEqual(skills["paths"], defaultPresetSkillsPaths) {
		t.Fatalf("skills.paths = %#v, want %#v", skills["paths"], defaultPresetSkillsPaths)
	}
	manifest := readManifestPortal(t, initPath)
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000", got)
	}
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("preset.active = %#v, want %q", got, want)
	}
}

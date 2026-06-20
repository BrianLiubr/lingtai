package migrate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPortalM039_LegacyRootContextCopiedToLLM(t *testing.T) {
	tmp := t.TempDir()
	lingtaiDir := filepath.Join(tmp, ".lingtai")
	initPath := writeAgentInitPortal(t, lingtaiDir, "alice", `{
  "manifest": {
    "context_limit": 300000,
    "llm": {"provider": "x", "model": "y"}
  }
}`)

	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	manifest := readManifestPortal(t, initPath)
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000", got)
	}
	if got := manifest["context_limit"]; got != float64(300000) {
		t.Fatalf("root context_limit = %#v, want preserved 300000", got)
	}
}

func TestPortalM039_LegacyRootContextCreatesMissingLLMBlock(t *testing.T) {
	tmp := t.TempDir()
	lingtaiDir := filepath.Join(tmp, ".lingtai")
	initPath := writeAgentInitPortal(t, lingtaiDir, "alice", `{
  "manifest": {
    "context_limit": 300000
  }
}`)

	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	manifest := readManifestPortal(t, initPath)
	llm, ok := manifest["llm"].(map[string]interface{})
	if !ok {
		t.Fatalf("llm block not created: %#v", manifest)
	}
	if got := llm["context_limit"]; got != float64(300000) {
		t.Fatalf("llm.context_limit = %#v, want 300000", got)
	}
}

func TestPortalM039_ExistingLLMContextWinsOnConflict(t *testing.T) {
	tmp := t.TempDir()
	lingtaiDir := filepath.Join(tmp, ".lingtai")
	initPath := writeAgentInitPortal(t, lingtaiDir, "bob", `{
  "manifest": {
    "context_limit": 300000,
    "llm": {"provider": "x", "model": "y", "context_limit": 128000}
  }
}`)

	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	manifest := readManifestPortal(t, initPath)
	llm := manifest["llm"].(map[string]interface{})
	if got := llm["context_limit"]; got != float64(128000) {
		t.Fatalf("llm.context_limit = %#v, want canonical 128000 preserved", got)
	}
}

func TestPortalM039_StalePresetMigratedWhenReplacementExists(t *testing.T) {
	tmp := t.TempDir()
	home := tmp
	t.Setenv("HOME", home)

	savedDir := filepath.Join(home, ".lingtai-tui", "presets", "saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir saved: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "codex-gpt5.5.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}

	lingtaiDir := filepath.Join(tmp, ".lingtai")
	initPath := writeAgentInitPortal(t, lingtaiDir, "carol", `{
  "manifest": {
    "llm": {"provider": "x", "model": "y", "context_limit": 300000},
    "preset": {
      "active": "~/.lingtai-tui/presets/saved/codex.json",
      "default": "~/.lingtai-tui/presets/saved/codex.json",
      "allowed": ["~/.lingtai-tui/presets/saved/codex.json"]
    }
  }
}`)

	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	manifest := readManifestPortal(t, initPath)
	preset := manifest["preset"].(map[string]interface{})
	want := "~/.lingtai-tui/presets/saved/codex-gpt5.5.json"
	if got := preset["active"]; got != want {
		t.Fatalf("active = %#v, want %q", got, want)
	}
	if got := preset["default"]; got != want {
		t.Fatalf("default = %#v, want %q", got, want)
	}
	allowed := preset["allowed"].([]interface{})
	if !reflect.DeepEqual(allowed, []interface{}{want}) {
		t.Fatalf("allowed = %#v, want [%q]", allowed, want)
	}
}

func TestPortalM039_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	savedDir := filepath.Join(tmp, ".lingtai-tui", "presets", "saved")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "codex-gpt5.5.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	lingtaiDir := filepath.Join(tmp, ".lingtai")
	initPath := writeAgentInitPortal(t, lingtaiDir, "erin", `{
  "manifest": {
    "context_limit": 300000,
    "llm": {"provider": "x", "model": "y"},
    "preset": {
      "active": "~/.lingtai-tui/presets/saved/codex.json",
      "default": "~/.lingtai-tui/presets/saved/codex.json",
      "allowed": ["~/.lingtai-tui/presets/saved/codex.json"]
    }
  }
}`)

	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate 1: %v", err)
	}
	after1, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("read after 1: %v", err)
	}
	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate 2: %v", err)
	}
	after2, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("read after 2: %v", err)
	}
	if string(after1) != string(after2) {
		t.Fatalf("second run changed file:\n--- 1 ---\n%s\n--- 2 ---\n%s", after1, after2)
	}
}

func TestPortalM039_SkipsNonAgentDirs(t *testing.T) {
	tmp := t.TempDir()
	lingtaiDir := filepath.Join(tmp, ".lingtai")

	humanInit := writeAgentInitPortal(t, lingtaiDir, "human", `{"manifest":{"context_limit":300000,"llm":{}}}`)
	dotInit := writeAgentInitPortal(t, lingtaiDir, ".portal", `{"manifest":{"context_limit":300000,"llm":{}}}`)

	humanBefore, _ := os.ReadFile(humanInit)
	dotBefore, _ := os.ReadFile(dotInit)

	if err := migrateAgentInitContextPresetRepair(lingtaiDir); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	humanAfter, _ := os.ReadFile(humanInit)
	dotAfter, _ := os.ReadFile(dotInit)
	if string(humanBefore) != string(humanAfter) {
		t.Fatalf("human init.json mutated: %s", humanAfter)
	}
	if string(dotBefore) != string(dotAfter) {
		t.Fatalf(".portal init.json mutated: %s", dotAfter)
	}
}

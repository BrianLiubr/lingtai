package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestCodex_CreateKnowledgeEntry_WritesFile(t *testing.T) {
	agentDir := t.TempDir()
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)

	if err := m.createKnowledgeEntry("MiMo notes", "How to choose MiMo endpoints"); err != nil {
		t.Fatalf("createKnowledgeEntry: %v", err)
	}

	path := filepath.Join(agentDir, "knowledge", "MiMo notes", "KNOWLEDGE.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	body := string(data)
	if !strings.Contains(body, "name: MiMo notes") {
		t.Errorf("missing name in frontmatter:\n%s", body)
	}
	if !strings.Contains(body, "description: How to choose MiMo endpoints") {
		t.Errorf("missing description:\n%s", body)
	}

	// Re-reading via the catalog builder should surface the new entry.
	entries := buildAgentCodexEntries(agentDir)
	if len(entries) != 1 || entries[0].Label != "MiMo notes" {
		t.Fatalf("after create: entries = %+v", entries)
	}
}

func TestCodex_CreateKnowledgeEntry_RejectsExistingFolder(t *testing.T) {
	agentDir := t.TempDir()
	folder := filepath.Join(agentDir, "knowledge", "dup")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)
	err := m.createKnowledgeEntry("dup", "anything")
	if err == nil {
		t.Fatal("expected error when folder already exists")
	}
}

func TestCodex_DeleteKnowledgeEntry_RemovesFolder(t *testing.T) {
	agentDir := t.TempDir()
	folder := filepath.Join(agentDir, "knowledge", "transient")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "KNOWLEDGE.md")
	if err := os.WriteFile(path, []byte("---\nname: t\ndescription: d\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewCodexModel(filepath.Dir(agentDir), agentDir)
	if err := m.deleteKnowledgeEntry(path); err != nil {
		t.Fatalf("deleteKnowledgeEntry: %v", err)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("expected folder removed, stat err = %v", err)
	}
}

func TestCodex_DeleteKnowledgeEntry_RefusesOutsideRoot(t *testing.T) {
	agentDir := t.TempDir()
	other := filepath.Join(t.TempDir(), "alien", "KNOWLEDGE.md")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)
	if err := m.deleteKnowledgeEntry(other); err == nil {
		t.Fatal("expected refusal for path outside knowledge root")
	}
}

func TestCodex_CreatePrompt_FullFlow(t *testing.T) {
	agentDir := t.TempDir()
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)
	m.width = 100

	// Send ctrl+n directly through CodexModel.Update. We bypass inner viewer
	// by triggering the Create message the same way ctrl+n does in mdviewer.
	var cmd tea.Cmd
	m, _ = m.Update(MarkdownViewerCreateMsg{})
	if m.prompt == nil {
		t.Fatal("expected prompt to open after MarkdownViewerCreateMsg")
	}

	// Type name + enter.
	for _, r := range "Alpha" {
		m, cmd = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		_ = cmd
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prompt == nil || m.prompt.fieldIdx != 1 {
		t.Fatalf("expected to advance to field 1, prompt=%+v", m.prompt)
	}
	// Type description + enter.
	for _, r := range "desc" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.prompt != nil {
		t.Fatalf("expected prompt cleared after submission, got %+v", m.prompt)
	}
	// File should exist.
	if _, err := os.Stat(filepath.Join(agentDir, "knowledge", "Alpha", "KNOWLEDGE.md")); err != nil {
		t.Fatalf("expected created file: %v", err)
	}
}

func TestCodex_DeletePrompt_RejectsLegacyEntry(t *testing.T) {
	agentDir := t.TempDir()
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)
	// Legacy content-only entry (no Path).
	legacy := MarkdownEntry{Label: "legacy", Group: "Legacy codex", Content: "x"}
	m, _ = m.Update(MarkdownViewerDeleteMsg{Index: 0, Entry: legacy})
	if m.prompt != nil {
		t.Fatal("legacy entry should not open delete prompt")
	}
}

func TestCodex_DeletePrompt_ConfirmYRemoves(t *testing.T) {
	agentDir := t.TempDir()
	folder := filepath.Join(agentDir, "knowledge", "k")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "KNOWLEDGE.md")
	if err := os.WriteFile(path, []byte("---\nname: k\ndescription: d\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)

	m, _ = m.Update(MarkdownViewerDeleteMsg{Index: 0, Entry: MarkdownEntry{Label: "k", Group: "Knowledge", Path: path}})
	if m.prompt == nil {
		t.Fatal("expected delete prompt to open")
	}
	// Press y.
	m, _ = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if m.prompt != nil {
		t.Fatalf("prompt should clear after y, got %+v", m.prompt)
	}
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Fatalf("folder should be removed, stat err = %v", err)
	}
}

func TestCodex_DeletePrompt_EscCancels(t *testing.T) {
	agentDir := t.TempDir()
	folder := filepath.Join(agentDir, "knowledge", "k")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "KNOWLEDGE.md")
	if err := os.WriteFile(path, []byte("---\nname: k\ndescription: d\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewCodexModel(filepath.Dir(agentDir), agentDir)
	m, _ = m.Update(MarkdownViewerDeleteMsg{Index: 0, Entry: MarkdownEntry{Label: "k", Group: "Knowledge", Path: path}})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.prompt != nil {
		t.Fatalf("esc should cancel, prompt = %+v", m.prompt)
	}
	if _, err := os.Stat(folder); err != nil {
		t.Fatalf("folder should still exist after cancel: %v", err)
	}
}

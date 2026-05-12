package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLibrary_CreateSkillEntry_WritesFile(t *testing.T) {
	agentDir := t.TempDir()
	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")

	if err := m.createSkillEntry("hello-skill", "say hi", "1.0.0"); err != nil {
		t.Fatalf("createSkillEntry: %v", err)
	}
	path := filepath.Join(agentDir, ".library", "custom", "hello-skill", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
	body := string(data)
	for _, want := range []string{"name: hello-skill", "description: say hi", "version: 1.0.0"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in SKILL.md:\n%s", want, body)
		}
	}
}

func TestLibrary_CreateSkillEntry_VersionOptional(t *testing.T) {
	agentDir := t.TempDir()
	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	if err := m.createSkillEntry("noversion", "no version skill", ""); err != nil {
		t.Fatalf("createSkillEntry: %v", err)
	}
	path := filepath.Join(agentDir, ".library", "custom", "noversion", "SKILL.md")
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "version:") {
		t.Errorf("did not expect version field when empty:\n%s", data)
	}
}

func TestLibrary_CreateSkillEntry_RejectsExistingFolder(t *testing.T) {
	agentDir := t.TempDir()
	dir := filepath.Join(agentDir, ".library", "custom", "dup")
	os.MkdirAll(dir, 0o755)
	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	if err := m.createSkillEntry("dup", "x", ""); err == nil {
		t.Fatal("expected error when folder exists")
	}
}

func TestLibrary_DeleteSkillEntry_RemovesFolder(t *testing.T) {
	agentDir := t.TempDir()
	dir := filepath.Join(agentDir, ".library", "custom", "todel")
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "SKILL.md")
	os.WriteFile(path, []byte("---\nname: todel\ndescription: x\n---\n"), 0o644)

	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	if err := m.deleteSkillEntry(path); err != nil {
		t.Fatalf("deleteSkillEntry: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("folder should be removed, stat err = %v", err)
	}
}

func TestLibrary_DeleteSkillEntry_RefusesNonCustomPath(t *testing.T) {
	agentDir := t.TempDir()
	intrinsicSkill := filepath.Join(agentDir, ".library", "intrinsic", "capabilities", "foo", "SKILL.md")
	os.MkdirAll(filepath.Dir(intrinsicSkill), 0o755)
	os.WriteFile(intrinsicSkill, []byte("x"), 0o644)
	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	if err := m.deleteSkillEntry(intrinsicSkill); err == nil {
		t.Fatal("expected refusal for intrinsic skill path")
	}
}

func TestLibrary_DeletePrompt_RejectsNonCustomGroup(t *testing.T) {
	agentDir := t.TempDir()
	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	entry := MarkdownEntry{Label: "intrinsic-x", Group: "capabilities", Path: "/tmp/x/SKILL.md"}
	m, _ = m.Update(MarkdownViewerDeleteMsg{Index: 0, Entry: entry})
	if m.prompt != nil {
		t.Fatal("non-custom group should not open delete prompt")
	}
}

func TestLibrary_CreatePrompt_FullFlow(t *testing.T) {
	agentDir := t.TempDir()
	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	m.width = 100

	m, _ = m.Update(MarkdownViewerCreateMsg{})
	if m.prompt == nil {
		t.Fatal("expected prompt open after create msg")
	}
	// Type name + enter
	for _, r := range "betaSkill" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// Type description + enter
	for _, r := range "describe me" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// Type version + enter
	for _, r := range "2.0.0" {
		m, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if m.prompt != nil {
		t.Fatalf("prompt should be cleared, got %+v", m.prompt)
	}
	path := filepath.Join(agentDir, ".library", "custom", "betaSkill", "SKILL.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected created skill: %v", err)
	}
}

func TestLibrary_DrillIn_BlocksCreatePrompt(t *testing.T) {
	agentDir := t.TempDir()
	// Make a custom skill so drill-in is meaningful.
	customDir := filepath.Join(agentDir, ".library", "custom", "x")
	os.MkdirAll(customDir, 0o755)
	os.WriteFile(filepath.Join(customDir, "SKILL.md"), []byte("---\nname: x\ndescription: d\n---\n"), 0o644)

	m := NewLibraryModel(filepath.Dir(agentDir), agentDir, "en")
	m.width = 100
	m.height = 30
	// Drill in by hand.
	files := buildSkillFolderEntries(customDir)
	sub := NewMarkdownViewer(files, "X")
	m.drillIn = &sub
	// Create message should be a no-op while drilled in.
	m, _ = m.Update(MarkdownViewerCreateMsg{})
	if m.prompt != nil {
		t.Fatal("create prompt must not open while drilled in")
	}
}

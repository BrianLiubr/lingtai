package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestMarkdownViewer_EmptyEntries(t *testing.T) {
	m := NewMarkdownViewer(nil, "Test")
	if len(m.entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(m.entries))
	}
}

func TestMarkdownViewer_CursorBounds(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "a", Group: "G", Content: "hello"},
		{Label: "b", Group: "G", Content: "world"},
	}
	m := NewMarkdownViewer(entries, "Test")
	// First group is expanded by default and cursor lands on the first
	// entry (row 1) rather than the group header (row 0).
	if m.cursor != 1 {
		t.Errorf("initial cursor = %d, want 1 (first entry, past group header)", m.cursor)
	}
	if idx := m.currentEntryIndex(); idx != 0 {
		t.Errorf("currentEntryIndex = %d, want 0", idx)
	}
}

func TestMarkdownViewer_ContentEntry(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "test", Group: "G", Content: "# Hello\n\nThis is content."},
	}
	m := NewMarkdownViewer(entries, "Test")
	m.width = 80
	m.height = 24
	right := m.renderRight(60)
	if right == "" {
		t.Error("renderRight returned empty for content entry")
	}
}

func TestMarkdownViewer_PathEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("# Test File\n\nContent here."), 0o644)

	entries := []MarkdownEntry{
		{Label: "test.md", Group: "G", Path: path},
	}
	m := NewMarkdownViewer(entries, "Test")
	m.width = 80
	m.height = 24
	right := m.renderRight(60)
	if right == "" {
		t.Error("renderRight returned empty for path entry")
	}
}

func TestMarkdownViewer_FrontmatterStripped(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "skill", Group: "G", Content: "---\nname: test\n---\n# Real Content"},
	}
	m := NewMarkdownViewer(entries, "Test")
	m.width = 80
	m.height = 24
	right := m.renderRight(60)
	if right == "" {
		t.Error("renderRight returned empty")
	}
	if strings.Contains(right, "name: test") {
		t.Error("frontmatter was not stripped")
	}
}

func TestMarkdownViewer_TreeDefaultExpansion(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "a", Group: "G1", Content: "x"},
		{Label: "b", Group: "G2", Content: "y"},
		{Label: "c", Group: "G2", Content: "z"},
	}
	m := NewMarkdownViewer(entries, "Test")
	if !m.expanded["G1"] {
		t.Error("first group should start expanded")
	}
	if m.expanded["G2"] {
		t.Error("non-first group should start collapsed")
	}
	// Visible nodes: G1 header, a (entry), G2 header. b/c hidden under collapsed G2.
	nodes := m.visibleNodes()
	if len(nodes) != 3 {
		t.Fatalf("visibleNodes len = %d, want 3 (G1, a, G2 collapsed); got %+v", len(nodes), nodes)
	}
	if !nodes[0].isGroup || nodes[0].group != "G1" {
		t.Errorf("nodes[0] = %+v, want G1 header", nodes[0])
	}
	if nodes[1].isGroup || nodes[1].entryIdx != 0 {
		t.Errorf("nodes[1] = %+v, want entry 0", nodes[1])
	}
	if !nodes[2].isGroup || nodes[2].group != "G2" {
		t.Errorf("nodes[2] = %+v, want G2 header", nodes[2])
	}
}

func TestMarkdownViewer_TreeToggleExpand(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "a", Group: "G1", Content: "x"},
		{Label: "b", Group: "G2", Content: "y"},
	}
	m := NewMarkdownViewer(entries, "Test")
	if got := len(m.visibleNodes()); got != 3 {
		t.Fatalf("before expand: visible = %d, want 3", got)
	}
	m.toggleGroup("G2")
	if !m.expanded["G2"] {
		t.Fatal("toggleGroup should have expanded G2")
	}
	if got := len(m.visibleNodes()); got != 4 {
		t.Fatalf("after expand: visible = %d, want 4 (G1, a, G2, b)", got)
	}
	m.toggleGroup("G1")
	if m.expanded["G1"] {
		t.Fatal("toggleGroup should have collapsed G1")
	}
	if got := len(m.visibleNodes()); got != 3 {
		t.Fatalf("after collapse G1: visible = %d, want 3 (G1, G2, b)", got)
	}
}

func TestMarkdownViewer_CursorSkipsCollapsed(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "a", Group: "G1", Content: "x"},
		{Label: "b", Group: "G2", Content: "y"},
		{Label: "c", Group: "G2", Content: "z"},
	}
	m := NewMarkdownViewer(entries, "Test")
	// cursor at 1 = entry "a". cursorLineInLeft should map to row 1.
	if got := m.cursorLineInLeft(); got != 1 {
		t.Errorf("cursorLineInLeft on entry a = %d, want 1", got)
	}
	// Move cursor down: G2 header is next visible row.
	m.cursor = 2
	nodes := m.visibleNodes()
	if !nodes[m.cursor].isGroup || nodes[m.cursor].group != "G2" {
		t.Errorf("cursor at 2 should be on G2 header; got %+v", nodes[m.cursor])
	}
	if idx := m.currentEntryIndex(); idx != -1 {
		t.Errorf("currentEntryIndex on group header = %d, want -1", idx)
	}
}

func TestMarkdownViewer_DescriptionRendered(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "alpha", Description: "first thing", Group: "G", Content: "x"},
		{Label: "beta", Description: "second thing", Group: "G", Content: "y"},
	}
	m := NewMarkdownViewer(entries, "Test")
	left := m.renderLeft(60)
	if !strings.Contains(left, "first thing") {
		t.Errorf("description 'first thing' missing from left panel:\n%s", left)
	}
	if !strings.Contains(left, "second thing") {
		t.Errorf("description 'second thing' missing from left panel:\n%s", left)
	}
	// Description subtitle counts as an extra line for cursor mapping.
	// cursor=1 → entry alpha. Line should be 1 (group header on 0, alpha on 1).
	if got := m.cursorLineInLeft(); got != 1 {
		t.Errorf("cursorLineInLeft = %d, want 1 (alpha row)", got)
	}
	// cursor=2 → entry beta, which sits two extra lines down (alpha's desc + beta).
	m.cursor = 2
	if got := m.cursorLineInLeft(); got != 3 {
		t.Errorf("cursorLineInLeft with description above = %d, want 3", got)
	}
}

func TestMarkdownViewer_TreeArrowIndicators(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "a", Group: "Expanded", Content: "x"},
		{Label: "b", Group: "Collapsed", Content: "y"},
	}
	m := NewMarkdownViewer(entries, "Test")
	left := m.renderLeft(60)
	if !strings.Contains(left, "▼") {
		t.Errorf("expected ▼ indicator for expanded group:\n%s", left)
	}
	if !strings.Contains(left, "▶") {
		t.Errorf("expected ▶ indicator for collapsed group:\n%s", left)
	}
}

func TestMarkdownViewer_CreateDeleteHintsRespectFlags(t *testing.T) {
	entries := []MarkdownEntry{{Label: "a", Group: "G", Content: "x"}}
	m := NewMarkdownViewer(entries, "T")
	m.width = 120
	m.height = 24
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})

	// Without flags, the footer should not mention ctrl+n / ctrl+d.
	view := m.View()
	if strings.Contains(view, "ctrl+n") || strings.Contains(view, "ctrl+d") {
		t.Errorf("flags off but found ctrl+n/ctrl+d in footer:\n%s", view)
	}

	m.EnableCreate = true
	m.EnableDelete = true
	view = m.View()
	if !strings.Contains(view, "ctrl+n") {
		t.Errorf("expected ctrl+n hint when EnableCreate=true:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+d") {
		t.Errorf("expected ctrl+d hint when EnableDelete=true:\n%s", view)
	}
}

func TestMarkdownViewer_CreateMsgEmittedOnCtrlN(t *testing.T) {
	entries := []MarkdownEntry{{Label: "a", Group: "G", Content: "x"}}
	m := NewMarkdownViewer(entries, "T")
	m.EnableCreate = true
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected cmd from ctrl+n")
	}
	msg := cmd()
	if _, ok := msg.(MarkdownViewerCreateMsg); !ok {
		t.Fatalf("expected MarkdownViewerCreateMsg, got %T", msg)
	}
}

func TestMarkdownViewer_DeleteMsgEmittedOnCtrlD(t *testing.T) {
	entries := []MarkdownEntry{{Label: "a", Group: "G", Content: "x", Path: "/tmp/x"}}
	m := NewMarkdownViewer(entries, "T")
	m.EnableDelete = true
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected cmd from ctrl+d")
	}
	msg := cmd()
	dm, ok := msg.(MarkdownViewerDeleteMsg)
	if !ok {
		t.Fatalf("expected MarkdownViewerDeleteMsg, got %T", msg)
	}
	if dm.Index != 0 {
		t.Errorf("Index = %d, want 0", dm.Index)
	}
	if dm.Entry.Label != "a" {
		t.Errorf("Entry.Label = %q, want a", dm.Entry.Label)
	}
}

func TestMarkdownViewer_SetEntriesRefreshes(t *testing.T) {
	entries := []MarkdownEntry{{Label: "a", Group: "G", Content: "x"}}
	m := NewMarkdownViewer(entries, "T")
	newEntries := []MarkdownEntry{
		{Label: "b", Group: "H", Content: "y"},
		{Label: "c", Group: "H", Content: "z"},
	}
	m.SetEntries(newEntries)
	if len(m.entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(m.entries))
	}
	if !m.expanded["H"] {
		t.Error("first group of new entries should be expanded by default")
	}
	if idx := m.currentEntryIndex(); idx != 0 {
		t.Errorf("currentEntryIndex after SetEntries = %d, want 0", idx)
	}
}

func TestMarkdownViewer_GroupRendering(t *testing.T) {
	entries := []MarkdownEntry{
		{Label: "a", Group: "Skills", Content: "x"},
		{Label: "b", Group: "Skills", Content: "y"},
		{Label: "c", Group: "Imported", Content: "z"},
	}
	m := NewMarkdownViewer(entries, "Test")
	m.width = 80
	m.height = 24
	left := m.renderLeft(30)
	if left == "" {
		t.Error("renderLeft returned empty")
	}
	if !strings.Contains(left, "Skills") {
		t.Error("missing Skills group header")
	}
	if !strings.Contains(left, "Imported") {
		t.Error("missing Imported group header")
	}
}

package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestDefaultCommandsIncludesNotification(t *testing.T) {
	cmd, ok := findCommand("notification")
	if !ok {
		t.Fatal("DefaultCommands() missing notification command")
	}
	if cmd.Description != "palette.notification" || cmd.Detail != "cmd.notification" {
		t.Fatalf("notification command keys = (%q, %q), want (palette.notification, cmd.notification)", cmd.Description, cmd.Detail)
	}
}

func TestNotificationCommandOpensNotificationView(t *testing.T) {
	agentDir := t.TempDir()
	app := App{orchDir: agentDir, projectDir: t.TempDir()}
	model, _ := app.switchToView("notification")
	got := model.(App)
	if got.currentView != appViewNotification {
		t.Fatalf("switchToView(%q) currentView = %v, want appViewNotification", "notification", got.currentView)
	}
	if got.notification.agentDir != agentDir {
		t.Fatalf("notification.agentDir = %q, want %q", got.notification.agentDir, agentDir)
	}
}

// TestNotificationModelNoSQLite checks graceful degradation when sqlite sidecar
// is absent: the model initializes without panic and View() returns a message.
func TestNotificationModelNoSQLite(t *testing.T) {
	agentDir := t.TempDir()
	m := NewNotificationModel(agentDir)
	if m.agentDir != agentDir {
		t.Fatalf("agentDir = %q, want %q", m.agentDir, agentDir)
	}
	view := m.View()
	if view == "" {
		t.Fatal("View() returned empty string")
	}
	// Should contain a hint about the missing sqlite file.
	if !strings.Contains(view, "log.sqlite") {
		t.Fatalf("View() did not mention log.sqlite: %s", view)
	}
}

// TestNotificationModelWithSQLite exercises navigation with a real sqlite file.
func TestNotificationModelWithSQLite(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires sqlite3 binary (POSIX only)")
	}
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not in PATH")
	}

	agentDir := t.TempDir()
	logsDir := filepath.Join(agentDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(logsDir, "log.sqlite")
	sql := `CREATE TABLE events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts REAL NOT NULL,
		type TEXT NOT NULL,
		agent_address TEXT,
		fields_json TEXT NOT NULL DEFAULT '{}',
		source_file TEXT,
		source_offset INTEGER,
		source_line INTEGER,
		source_kind TEXT,
		scope TEXT,
		run_id TEXT,
		inserted_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);
	INSERT INTO events(ts,type,fields_json) VALUES(1000.0,'notification_pair_injected','{"sources":["email"],"summary":"first"}');
	INSERT INTO events(ts,type,fields_json) VALUES(1001.0,'email_notification_published','{"count":2}');
	INSERT INTO events(ts,type,fields_json) VALUES(1002.0,'notification_pair_injected','{"sources":["email"],"summary":"third"}');`
	if out, err := exec.Command(bin, db, sql).CombinedOutput(); err != nil {
		t.Fatalf("createDB: %v\n%s", err, out)
	}

	m := NewNotificationModel(agentDir)
	// Should start at newest event.
	if m.current == nil {
		t.Fatal("current should be set after init with real sqlite")
	}
	if !strings.Contains(m.current.FieldsJSON, "third") {
		t.Fatalf("expected newest event first, got fields_json=%s", m.current.FieldsJSON)
	}
	if m.total != 3 {
		t.Fatalf("total = %d, want 3", m.total)
	}

	// Navigate to older event.
	m.stepOlder()
	if m.current == nil {
		t.Fatal("current nil after stepOlder")
	}
	if !strings.Contains(m.current.FieldsJSON, "count") {
		t.Fatalf("expected middle event after stepOlder, got fields_json=%s", m.current.FieldsJSON)
	}

	// Navigate back to newer.
	m.stepNewer()
	if !strings.Contains(m.current.FieldsJSON, "third") {
		t.Fatalf("expected newest after stepNewer, got fields_json=%s", m.current.FieldsJSON)
	}

	// View renders without error.
	m.width = 100
	m.height = 30
	view := m.View()
	if !strings.Contains(view, "notification_pair_injected") {
		t.Fatalf("View() does not show event type: %s", view)
	}
	if !strings.Contains(view, "third") {
		t.Fatalf("View() does not show fields_json content: %s", view)
	}
}

// TestNotificationModelKeyNavigation checks that Update dispatches left/right keys.
func TestNotificationModelKeyNavigation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires sqlite3 binary (POSIX only)")
	}
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not in PATH")
	}

	agentDir := t.TempDir()
	logsDir := filepath.Join(agentDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(logsDir, "log.sqlite")
	sql := `CREATE TABLE events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts REAL NOT NULL,
		type TEXT NOT NULL,
		agent_address TEXT,
		fields_json TEXT NOT NULL DEFAULT '{}',
		source_file TEXT,
		source_offset INTEGER,
		source_line INTEGER,
		source_kind TEXT,
		scope TEXT,
		run_id TEXT,
		inserted_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	);
	INSERT INTO events(ts,type,fields_json) VALUES(1000.0,'notification_pair_injected','{"n":1}');
	INSERT INTO events(ts,type,fields_json) VALUES(1001.0,'notification_pair_injected','{"n":2}');`
	if out, err := exec.Command(bin, db, sql).CombinedOutput(); err != nil {
		t.Fatalf("createDB: %v\n%s", err, out)
	}

	m := NewNotificationModel(agentDir)
	if m.current == nil {
		t.Fatal("no current event after init")
	}
	startID := m.current.ID

	// left key → older (lower id)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.current == nil {
		t.Fatal("current nil after left key")
	}
	if m.current.ID >= startID {
		t.Fatalf("left key should move to older event (lower id), got id=%d start=%d", m.current.ID, startID)
	}
	afterLeft := m.current.ID

	// right key → back to newer
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.current.ID <= afterLeft {
		t.Fatalf("right key should move to newer event, got id=%d prev=%d", m.current.ID, afterLeft)
	}
}

package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// loadDetail counts avatar spawn entries from delegates/ledger.jsonl.
// Regression test for issue #196: the count must reflect avatar/delegate
// spawns (event == "avatar"), not every non-empty line in the ledger.
func TestLoadDetailCountsOnlyAvatarSpawns(t *testing.T) {
	agentDir := filepath.Join(t.TempDir(), "alice")
	delegatesDir := filepath.Join(agentDir, "delegates")
	if err := os.MkdirAll(delegatesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Two avatar spawns interleaved with a non-avatar event and a blank
	// line. Only the two avatar records should be counted.
	ledger := `{"event":"avatar","name":"bob","working_dir":"bob","ts":1000}
{"event":"heartbeat","ts":1500}

{"event":"avatar","name":"carol","working_dir":"carol","ts":2000}
`
	if err := os.WriteFile(filepath.Join(delegatesDir, "ledger.jsonl"), []byte(ledger), 0o644); err != nil {
		t.Fatalf("write ledger: %v", err)
	}

	m := PropsModel{selectedDir: agentDir}
	m.loadDetail()

	if m.detailAvatarCount != 2 {
		t.Errorf("detailAvatarCount = %d, want 2 (only avatar records)", m.detailAvatarCount)
	}
}

// A missing ledger yields a zero count without error.
func TestLoadDetailNoLedger(t *testing.T) {
	agentDir := filepath.Join(t.TempDir(), "alice")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	m := PropsModel{selectedDir: agentDir}
	m.loadDetail()

	if m.detailAvatarCount != 0 {
		t.Errorf("detailAvatarCount = %d, want 0 for missing ledger", m.detailAvatarCount)
	}
}

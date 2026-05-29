package tui

import (
	"strings"
	"testing"
)

func TestRenderDoctorIntrinsicReportMapsSeveritiesAndHints(t *testing.T) {
	json := []byte(`{
		"severity":"WARN",
		"sections":[{"name":"mcp/addons","severity":"FAIL","findings":[
			{"severity":"FAIL","title":"telegram: stdio command","detail":"Command path/executable could not be found."},
			{"severity":"OK","title":"notification directory scanned","detail":"Found 0 notification file(s)."},
			{"severity":"WARN","title":"logs stale","detail":"Heartbeat is fresh but logs are old."}
		]}],
		"next_steps":["Back up init.json, update stale command paths, then refresh."]
	}`)
	lines, err := renderDoctorIntrinsicReport(json)
	if err != nil {
		t.Fatalf("renderDoctorIntrinsicReport: %v", err)
	}
	if len(lines) < 6 {
		t.Fatalf("expected rendered lines, got %d", len(lines))
	}
	if !lines[0].Warn || !strings.Contains(lines[0].Text, "WARN") {
		t.Fatalf("summary line not warn: %#v", lines[0])
	}
	if lines[1].OK || lines[1].Warn || !strings.Contains(lines[1].Text, "mcp/addons") {
		t.Fatalf("section fail line not red/default: %#v", lines[1])
	}
	if !lines[3].OK || !strings.Contains(lines[3].Text, "notification directory") {
		t.Fatalf("OK finding not mapped green: %#v", lines[3])
	}
	if !lines[4].Warn || !strings.Contains(lines[4].Text, "logs stale") {
		t.Fatalf("WARN finding not mapped amber: %#v", lines[4])
	}
	last := lines[len(lines)-1]
	if !last.Hint || !strings.Contains(last.Text, "Back up init.json") {
		t.Fatalf("next step not rendered as hint: %#v", last)
	}
}

func TestRenderDoctorIntrinsicReportMalformedJSON(t *testing.T) {
	if _, err := renderDoctorIntrinsicReport([]byte(`not json`)); err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

func TestLineForDoctorSeverityUnknown(t *testing.T) {
	line := lineForDoctorSeverity("ODD", "strange")
	if !line.Warn || !strings.Contains(line.Text, "strange") {
		t.Fatalf("unknown severity should be warn/info line: %#v", line)
	}
}

func TestTruncateDoctorDetail(t *testing.T) {
	short := "small"
	if truncateDoctorDetail(short) != short {
		t.Fatal("short detail should be unchanged")
	}
	long := strings.Repeat("x", 300)
	got := truncateDoctorDetail(long)
	if len(got) >= len(long) || !strings.HasSuffix(got, "...") {
		t.Fatalf("long detail not truncated: len=%d suffix=%q", len(got), got[len(got)-3:])
	}
}

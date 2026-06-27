package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSelectTUIUpdaterRoutesInstallMethods(t *testing.T) {
	tests := []struct {
		name   string
		method TUIInstallMethod
		want   TUIInstallMethod
	}{
		{name: "homebrew", method: TUIInstallMethodHomebrew, want: TUIInstallMethodHomebrew},
		{name: "source", method: TUIInstallMethodSource, want: TUIInstallMethodSource},
		{name: "unknown", method: TUIInstallMethodUnknown, want: TUIInstallMethodUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updater := SelectTUIUpdater(TUIInstallInfo{Method: tc.method})
			if got := updater.InstallMethod(); got != tc.want {
				t.Fatalf("InstallMethod() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestHomebrewTUIUpdaterDoctorRunsUpdateThenUpgrade(t *testing.T) {
	runner := &fakeRunner{}
	result := RunTUIUpdate(TUIInstallInfo{Method: TUIInstallMethodHomebrew}, TUIUpdateOptions{
		LatestVersion:         "v0.8.1",
		Runner:                runner,
		LookPath:              func(string) (string, error) { return "/opt/homebrew/bin/brew", nil },
		IncludeHomebrewUpdate: true,
		ResolveHomebrewPath:   true,
	})

	if !result.Healthy || !result.Updated {
		t.Fatalf("expected healthy updated result: %+v", result)
	}
	update := "/opt/homebrew/bin/brew update"
	upgrade := "/opt/homebrew/bin/brew upgrade lingtai-ai/lingtai/lingtai-tui"
	if !containsCall(runner.calls, update) || !containsCall(runner.calls, upgrade) {
		t.Fatalf("expected doctor brew calls, got %#v", runner.calls)
	}
	if indexOfCall(runner.calls, update) > indexOfCall(runner.calls, upgrade) {
		t.Fatalf("expected brew update before brew upgrade, got %#v", runner.calls)
	}
	if !containsLine(result.Lines, "Brew upgrade finished") {
		t.Fatalf("expected restart guidance line: %+v", result.Lines)
	}
}

func TestHomebrewTUIUpdaterStartupRunsUpgradeOnlyWithoutLookPath(t *testing.T) {
	runner := &fakeRunner{}
	lookedUp := false
	result := RunTUIUpdate(TUIInstallInfo{Method: TUIInstallMethodHomebrew}, TUIUpdateOptions{
		LatestVersion: "v0.8.1",
		Runner:        runner,
		LookPath: func(string) (string, error) {
			lookedUp = true
			return "", errors.New("should not be called")
		},
	})

	if !result.Healthy || !result.Updated {
		t.Fatalf("expected healthy updated result: %+v", result)
	}
	if lookedUp {
		t.Fatalf("startup-style updater should execute brew by name without resolving PATH")
	}
	if len(runner.calls) != 1 || runner.calls[0] != "brew upgrade lingtai-ai/lingtai/lingtai-tui" {
		t.Fatalf("expected only brew upgrade call, got %#v", runner.calls)
	}
}

func TestTUIUpdaterSourceAndUnknownDoNotRunBrew(t *testing.T) {
	tests := []struct {
		name       string
		method     TUIInstallMethod
		wantLine   string
		wantHealth bool
	}{
		{name: "source", method: TUIInstallMethodSource, wantLine: "Source/user-local TUI update is not automated yet", wantHealth: true},
		{name: "unknown", method: TUIInstallMethodUnknown, wantLine: "TUI install method is unknown", wantHealth: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{}
			result := RunTUIUpdate(TUIInstallInfo{Method: tc.method}, TUIUpdateOptions{
				LatestVersion: "v0.8.1",
				Runner:        runner,
				LookPath:      func(string) (string, error) { return "/opt/homebrew/bin/brew", nil },
			})
			if result.Healthy != tc.wantHealth {
				t.Fatalf("Healthy = %v, want %v: %+v", result.Healthy, tc.wantHealth, result.Lines)
			}
			if !containsLine(result.Lines, tc.wantLine) {
				t.Fatalf("expected guidance line %q, got %+v", tc.wantLine, result.Lines)
			}
			if containsCall(runner.calls, "brew") {
				t.Fatalf("%s updater must not run brew, got %#v", tc.name, runner.calls)
			}
		})
	}
}

func TestManualTUIUpdateHomebrewRoutesThroughUpdater(t *testing.T) {
	runner := &fakeRunner{}
	report := RunManualTUIUpdate(t.TempDir(), ManualTUIUpdateOptions{
		CurrentTUIVersion: "v0.8.0",
		HTTPClient:        testVersionClient(t, "0.9.7", "v0.8.1"),
		Runner:            runner,
		LookPath: func(name string) (string, error) {
			if name == "brew" {
				return "/opt/homebrew/bin/brew", nil
			}
			return "", errors.New("not found")
		},
		Executable: func() (string, error) { return "/opt/homebrew/bin/lingtai-tui", nil },
		LookupEnv:  func(string) (string, bool) { return "", false },
	})

	if !report.Healthy || !report.Updated {
		t.Fatalf("expected healthy updated result: %+v", report)
	}
	if !containsLine(report.Lines, "TUI install method: homebrew") {
		t.Fatalf("expected homebrew install method line: %+v", report.Lines)
	}
	if !containsLine(report.Lines, "TUI update available: v0.8.0 -> v0.8.1") {
		t.Fatalf("expected update-available line: %+v", report.Lines)
	}
	update := "/opt/homebrew/bin/brew update"
	upgrade := "/opt/homebrew/bin/brew upgrade lingtai-ai/lingtai/lingtai-tui"
	if !containsCall(runner.calls, update) || !containsCall(runner.calls, upgrade) {
		t.Fatalf("expected manual brew calls, got %#v", runner.calls)
	}
	if indexOfCall(runner.calls, update) > indexOfCall(runner.calls, upgrade) {
		t.Fatalf("expected brew update before brew upgrade, got %#v", runner.calls)
	}
}

func TestManualTUIUpdateSourceInstallIsUnsupported(t *testing.T) {
	globalDir := t.TempDir()
	prefix := t.TempDir()
	binDir := filepath.Join(prefix, "bin")
	exe := filepath.Join(binDir, "lingtai-tui")
	writeSourceInstallMetadata(t, globalDir, prefix, binDir, []string{exe})
	runner := &fakeRunner{}

	report := RunManualTUIUpdate(globalDir, ManualTUIUpdateOptions{
		CurrentTUIVersion: "v0.8.0",
		HTTPClient:        testVersionClient(t, "0.9.7", "v0.8.1"),
		Runner:            runner,
		LookPath:          func(string) (string, error) { return "/opt/homebrew/bin/brew", nil },
		Executable:        func() (string, error) { return exe, nil },
		LookupEnv:         func(string) (string, bool) { return "", false },
	})

	if report.Healthy || report.Updated {
		t.Fatalf("source manual self-update should be unsupported: %+v", report)
	}
	if !containsLine(report.Lines, "TUI install method: source/user-local") {
		t.Fatalf("expected source install method line: %+v", report.Lines)
	}
	if !containsLine(report.Lines, "Source/user-local TUI update is not automated yet") {
		t.Fatalf("expected source updater guidance: %+v", report.Lines)
	}
	if !containsLine(report.Lines, "Manual self-update for source/user-local") {
		t.Fatalf("expected unsupported manual command line: %+v", report.Lines)
	}
	if containsCall(runner.calls, "brew") {
		t.Fatalf("source manual self-update must not run brew, got %#v", runner.calls)
	}
}

func TestManualTUIUpdateUnknownInstallIsUnsupported(t *testing.T) {
	runner := &fakeRunner{}
	report := RunManualTUIUpdate(t.TempDir(), ManualTUIUpdateOptions{
		CurrentTUIVersion: "v0.8.0",
		HTTPClient:        testVersionClient(t, "0.9.7", "v0.8.1"),
		Runner:            runner,
		LookPath:          func(string) (string, error) { return "/opt/homebrew/bin/brew", nil },
		Executable:        func() (string, error) { return "/tmp/manual/lingtai-tui", nil },
		LookupEnv:         func(string) (string, bool) { return "", false },
	})

	if report.Healthy || report.Updated {
		t.Fatalf("unknown manual self-update should be unsupported: %+v", report)
	}
	if !containsLine(report.Lines, "TUI install method: unknown/other") {
		t.Fatalf("expected unknown install method line: %+v", report.Lines)
	}
	if !containsLine(report.Lines, "TUI install method is unknown") {
		t.Fatalf("expected unknown updater guidance: %+v", report.Lines)
	}
	if containsCall(runner.calls, "brew") {
		t.Fatalf("unknown manual self-update must not run brew, got %#v", runner.calls)
	}
}

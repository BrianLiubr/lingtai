package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/lingtai-tui/i18n"
	"github.com/anthropics/lingtai-tui/internal/config"
)

const doctorIntrinsicTimeout = 15 * time.Second

type doctorIntrinsicReport struct {
	Severity  string                   `json:"severity"`
	Sections  []doctorIntrinsicSection `json:"sections"`
	NextSteps []string                 `json:"next_steps"`
}

type doctorIntrinsicSection struct {
	Name     string                   `json:"name"`
	Severity string                   `json:"severity"`
	Findings []doctorIntrinsicFinding `json:"findings"`
}

type doctorIntrinsicFinding struct {
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

func runKernelDoctorIntrinsic(orchDir, globalDir string) []doctorLine {
	python := config.LingtaiCmd(globalDir)
	if _, err := os.Stat(python); err != nil {
		return []doctorLine{
			{Text: i18n.TF("doctor.intrinsic_unavailable", err.Error()), Warn: true},
			{Text: i18n.T("doctor.intrinsic_hint"), Hint: true},
		}
	}

	scriptPath, err := resolveDoctorScriptPath(python)
	if err != nil {
		return []doctorLine{{Text: i18n.TF("doctor.intrinsic_unavailable", err.Error()), Warn: true}}
	}

	jsonBytes, stderr, err := runDoctorScript(python, scriptPath, orchDir, doctorIntrinsicTimeout)
	if len(jsonBytes) == 0 {
		detail := strings.TrimSpace(stderr)
		if detail == "" && err != nil {
			detail = err.Error()
		}
		if detail == "" {
			detail = "no output"
		}
		return []doctorLine{
			{Text: i18n.TF("doctor.intrinsic_unavailable", truncateDoctorDetail(detail)), Warn: true},
			{Text: i18n.T("doctor.intrinsic_hint"), Hint: true},
		}
	}

	lines, parseErr := renderDoctorIntrinsicReport(jsonBytes)
	if parseErr != nil {
		detail := parseErr.Error()
		if stderr = strings.TrimSpace(stderr); stderr != "" {
			detail += ": " + stderr
		}
		return []doctorLine{
			{Text: i18n.TF("doctor.intrinsic_unavailable", truncateDoctorDetail(detail)), Warn: true},
			{Text: i18n.T("doctor.intrinsic_hint"), Hint: true},
		}
	}
	return lines
}

func resolveDoctorScriptPath(python string) (string, error) {
	cmd := exec.Command(python, "-c", "import lingtai, pathlib; print(pathlib.Path(lingtai.__file__).parent / 'intrinsic_skills/lingtai-doctor/scripts/doctor.py')")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("cannot locate kernel intrinsic: %s", truncateDoctorDetail(detail))
	}
	path := strings.TrimSpace(stdout.String())
	if path == "" {
		return "", fmt.Errorf("kernel intrinsic path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("kernel intrinsic script missing: %s", path)
	}
	return path, nil
}

func runDoctorScript(python, scriptPath, orchDir string, timeout time.Duration) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, scriptPath, "--agent-dir", orchDir, "--json")
	cmd.Env = append(os.Environ(), "LINGTAI_AGENT_DIR="+orchDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return stdout.Bytes(), stderr.String(), fmt.Errorf("timed out after %s", timeout)
	}
	return stdout.Bytes(), stderr.String(), err
}

func renderDoctorIntrinsicReport(jsonBytes []byte) ([]doctorLine, error) {
	var report doctorIntrinsicReport
	if err := json.Unmarshal(jsonBytes, &report); err != nil {
		return nil, fmt.Errorf("cannot parse intrinsic JSON: %w", err)
	}
	var lines []doctorLine
	if report.Severity != "" {
		lines = append(lines, doctorLine{Text: i18n.TF("doctor.intrinsic_summary", report.Severity), Warn: report.Severity == "WARN", OK: report.Severity == "OK"})
	}
	for _, section := range report.Sections {
		if section.Name == "" {
			continue
		}
		lines = append(lines, lineForDoctorSeverity(section.Severity, fmt.Sprintf("%s [%s]", section.Name, section.Severity)))
		for _, finding := range section.Findings {
			text := finding.Title
			if finding.Detail != "" {
				text += ": " + finding.Detail
			}
			if text == "" {
				continue
			}
			lines = append(lines, lineForDoctorSeverity(finding.Severity, "  "+text))
		}
	}
	for _, step := range report.NextSteps {
		if strings.TrimSpace(step) != "" {
			lines = append(lines, doctorLine{Text: "→ " + step, Hint: true})
		}
	}
	return lines, nil
}

func lineForDoctorSeverity(severity, text string) doctorLine {
	switch severity {
	case "OK":
		return doctorLine{Text: "✓ " + text, OK: true}
	case "WARN":
		return doctorLine{Text: "! " + text, Warn: true}
	case "FAIL":
		return doctorLine{Text: "✗ " + text}
	default:
		return doctorLine{Text: "• " + text, Warn: true}
	}
}

func truncateDoctorDetail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 240 {
		return s
	}
	return s[:240] + "..."
}

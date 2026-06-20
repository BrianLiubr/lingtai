package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/anthropics/lingtai-tui/i18n"
	"github.com/anthropics/lingtai-tui/internal/sqlitelog"
)

// NotificationModel is the /notification view: a just-in-time history browser
// over the current agent's logs/log.sqlite. Left/right keys step through
// notification-related events by id without preloading or caching.
type NotificationModel struct {
	agentDir string
	width    int
	height   int

	// current event being displayed; nil means "show index / no event selected"
	current *sqlitelog.NotificationEvent

	// total count loaded on open (best-effort; 0 when unavailable)
	total int

	// error from last query (shown inline)
	err string

	// statusLine is the one-line footer hint
	statusLine string
}

// NewNotificationModel creates the /notification model for agentDir.
// It immediately queries for the latest notification event.
func NewNotificationModel(agentDir string) NotificationModel {
	m := NotificationModel{agentDir: agentDir}
	m.loadLatest()
	return m
}

func (m *NotificationModel) loadLatest() {
	if m.agentDir == "" {
		m.err = "No agent selected."
		return
	}
	if !sqlitelog.Exists(m.agentDir) {
		m.err = "logs/log.sqlite not found. Run `lingtai-agent log rebuild <agent_dir>` to create it."
		return
	}
	events, err := sqlitelog.QueryNotifications(m.agentDir, 50)
	if err != nil {
		m.err = fmt.Sprintf("query error: %v", err)
		return
	}
	m.err = ""
	m.total = len(events)
	if len(events) == 0 {
		m.current = nil
		return
	}
	// Start at newest (index 0 = highest id in DESC order).
	m.current = &events[0]
}

func (m *NotificationModel) stepOlder() {
	if m.current == nil {
		return
	}
	prev, err := sqlitelog.QueryNotificationBefore(m.agentDir, m.current.ID)
	if err != nil {
		m.err = fmt.Sprintf("query error: %v", err)
		return
	}
	m.err = ""
	if prev != nil {
		m.current = prev
	}
}

func (m *NotificationModel) stepNewer() {
	if m.current == nil {
		return
	}
	next, err := sqlitelog.QueryNotificationAfter(m.agentDir, m.current.ID)
	if err != nil {
		m.err = fmt.Sprintf("query error: %v", err)
		return
	}
	m.err = ""
	if next != nil {
		m.current = next
	}
}

func (m *NotificationModel) reload() {
	m.loadLatest()
}

func (m NotificationModel) Init() tea.Cmd { return nil }

func (m NotificationModel) Update(msg tea.Msg) (NotificationModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "left":
			m.stepOlder()
		case "right":
			m.stepNewer()
		case "ctrl+r", "r":
			m.reload()
		}
	}
	return m, nil
}

func (m NotificationModel) View() string {
	title := notificationTitle(m.agentDir)
	hint := StyleFaint.Render("← older  → newer  r reload  esc back")

	if m.err != "" {
		body := StyleSubtle.Render(m.err)
		return renderNotificationPanel(title, body, hint, m.width, m.height)
	}

	if m.current == nil {
		body := StyleSubtle.Render("No notification events found in logs/log.sqlite.")
		return renderNotificationPanel(title, body, hint, m.width, m.height)
	}

	body := renderNotificationEvent(*m.current, m.total)
	return renderNotificationPanel(title, body, hint, m.width, m.height)
}

func notificationTitle(agentDir string) string {
	base := i18n.T("palette.notification")
	if agentDir == "" {
		return base
	}
	return fmt.Sprintf("%s — %s", base, filepath.Base(agentDir))
}

// renderNotificationEvent formats a single notification event for display.
func renderNotificationEvent(ev sqlitelog.NotificationEvent, total int) string {
	var b strings.Builder

	// Header row
	tsStr := ev.Time().Format(time.RFC3339)
	typeStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	idStyle := StyleFaint

	fmt.Fprintf(&b, "%s  %s  %s\n",
		typeStyle.Render(ev.Type),
		idStyle.Render(fmt.Sprintf("id=%d", ev.ID)),
		StyleSubtle.Render(tsStr),
	)

	if ev.Source != "" && ev.Source != "." {
		fmt.Fprintf(&b, "%s\n", StyleFaint.Render("source: "+ev.Source))
	}

	b.WriteString("\n")

	// Fields JSON, pretty-printed
	pretty := sqlitelog.PrettyFields(ev)
	b.WriteString(pretty)
	b.WriteString("\n")

	if total > 0 {
		b.WriteString("\n")
		b.WriteString(StyleFaint.Render(fmt.Sprintf("%d notification event(s) in history", total)))
	}

	return b.String()
}

// renderNotificationPanel wraps content in a simple titled box.
func renderNotificationPanel(title, body, hint string, width, height int) string {
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	divider := StyleFaint.Render(strings.Repeat("─", max(0, width-4)))

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n")
	b.WriteString(body)

	// Pad to height-2 so the hint sticks to the bottom.
	lines := strings.Count(b.String(), "\n") + 1
	pad := height - lines - 2
	if pad > 0 {
		b.WriteString(strings.Repeat("\n", pad))
	}
	b.WriteString("\n")
	b.WriteString(hint)

	return b.String()
}

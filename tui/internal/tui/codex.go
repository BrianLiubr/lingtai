package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/anthropics/lingtai-tui/i18n"
	"github.com/anthropics/lingtai-tui/internal/fs"
)

// CodexModel is the top-level /knowledge view. Mirrors LibraryModel: shows one
// agent's private knowledge at a time and swaps agents via Ctrl+T.
type CodexModel struct {
	baseDir     string // .lingtai/ directory (for agent discovery)
	selectedDir string // working dir of the currently-displayed agent

	inner MarkdownViewerModel

	pickerOpen bool
	pickerIdx  int
	agentNodes []fs.AgentNode

	// Inline create / delete prompt. Non-nil while the prompt UI is active;
	// drives the footer block and absorbs keys until done / cancelled.
	prompt *entryPrompt

	width  int
	height int
	ready  bool

	pickerVP viewport.Model
}

type codexLoadMsg struct {
	agentNodes []fs.AgentNode
}

// NewCodexModel constructs the /knowledge view rooted at baseDir with the given
// agent pre-selected.
func NewCodexModel(baseDir, selectedDir string) CodexModel {
	entries := buildAgentCodexEntries(selectedDir)
	inner := NewMarkdownViewer(entries, codexTitleFor(selectedDir))
	inner.FooterHint = i18n.T("hints.props_select")
	inner.EnableCreate = true
	inner.EnableDelete = true
	return CodexModel{
		baseDir:     baseDir,
		selectedDir: selectedDir,
		inner:       inner,
	}
}

func codexTitleFor(agentDir string) string {
	base := i18n.T("palette.knowledge")
	if agentDir == "" {
		return base
	}
	name := filepath.Base(agentDir)
	if manifest, err := fs.ReadInitManifest(agentDir); err == nil {
		if v, ok := manifest["nickname"].(string); ok && v != "" {
			name = v
		} else if v, ok := manifest["agent_name"].(string); ok && v != "" {
			name = v
		}
	}
	return fmt.Sprintf("%s — %s", base, name)
}

func (m CodexModel) loadAgents() tea.Msg {
	net, _ := fs.BuildNetwork(m.baseDir)
	var nodes []fs.AgentNode
	for _, n := range net.Nodes {
		if n.IsHuman {
			continue
		}
		if n.WorkingDir == "" {
			continue
		}
		nodes = append(nodes, n)
	}
	return codexLoadMsg{agentNodes: nodes}
}

func (m CodexModel) Init() tea.Cmd {
	return tea.Batch(m.inner.Init(), m.loadAgents)
}

const (
	codexHeaderLines = 2
	codexFooterLines = 2
)

func (m CodexModel) Update(msg tea.Msg) (CodexModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		vpHeight := m.height - codexHeaderLines - codexFooterLines
		if vpHeight < 1 {
			vpHeight = 1
		}
		if !m.ready {
			m.pickerVP = viewport.New()
			m.ready = true
		}
		m.pickerVP.SetWidth(m.width)
		m.pickerVP.SetHeight(vpHeight)
		m.syncPicker()
		var cmd tea.Cmd
		m.inner, cmd = m.inner.Update(msg)
		return m, cmd

	case codexLoadMsg:
		m.agentNodes = msg.agentNodes
		found := false
		for _, n := range m.agentNodes {
			if n.WorkingDir == m.selectedDir {
				found = true
				break
			}
		}
		if !found && len(m.agentNodes) > 0 {
			m.pickerIdx = 0
		}
		return m, nil

	case MarkdownViewerCreateMsg:
		p := newCreatePrompt(i18n.T("mdviewer.create_knowledge_title"), []entryPromptField{
			{Key: "name", Label: i18n.T("mdviewer.field_name"), Placeholder: i18n.T("mdviewer.field_name_ph"), Required: true, CharLimit: 80},
			{Key: "description", Label: i18n.T("mdviewer.field_description"), Placeholder: i18n.T("mdviewer.field_description_ph"), Required: true, CharLimit: 300},
		})
		p.SetWidth(m.width)
		m.prompt = &p
		return m, nil

	case MarkdownViewerDeleteMsg:
		// All knowledge entries are deletable iff they're path-backed (i.e.
		// not the legacy codex.json fallback, which produces Content-only
		// entries).
		if msg.Entry.Path == "" {
			m.inner.SetStatus(i18n.T("mdviewer.delete_legacy_unsupported"), true)
			return m, nil
		}
		p := newDeleteConfirmPrompt(i18n.T("mdviewer.delete_knowledge_title"), msg.Entry.Label)
		// Stash the path in the values so the handler knows what to remove.
		p.values["path"] = msg.Entry.Path
		m.prompt = &p
		return m, nil

	case tea.KeyPressMsg:
		if m.pickerOpen {
			return m.updatePicker(msg)
		}
		if m.prompt != nil {
			return m.updatePrompt(msg)
		}
		switch msg.String() {
		case "ctrl+t":
			if len(m.agentNodes) == 0 {
				return m, nil
			}
			m.pickerOpen = true
			m.pickerIdx = 0
			for i, n := range m.agentNodes {
				if n.WorkingDir == m.selectedDir {
					m.pickerIdx = i
					break
				}
			}
			m.syncPicker()
			return m, nil
		}
		var cmd tea.Cmd
		m.inner, cmd = m.inner.Update(msg)
		return m, cmd

	case tea.MouseWheelMsg:
		if m.pickerOpen {
			var cmd tea.Cmd
			m.pickerVP, cmd = m.pickerVP.Update(msg)
			return m, cmd
		}
		if m.prompt != nil {
			return m, nil
		}
		var cmd tea.Cmd
		m.inner, cmd = m.inner.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.inner, cmd = m.inner.Update(msg)
	return m, cmd
}

// updatePrompt processes a key while the inline create / delete prompt is
// active. Returns once the prompt completes (Done or Cancelled) and applies
// the resulting mutation to disk.
func (m CodexModel) updatePrompt(msg tea.KeyPressMsg) (CodexModel, tea.Cmd) {
	if m.prompt == nil {
		return m, nil
	}
	p, cmd := m.prompt.Update(msg)
	m.prompt = &p
	if p.Cancelled() {
		m.prompt = nil
		return m, cmd
	}
	if !p.Done() {
		return m, cmd
	}
	// Prompt complete — perform the mutation.
	values := p.Values()
	wasCreate := p.kind == entryPromptCreate
	m.prompt = nil
	if wasCreate {
		if err := m.createKnowledgeEntry(values["name"], values["description"]); err != nil {
			m.inner.SetStatus(i18n.TF("mdviewer.create_failed", err.Error()), true)
		} else {
			m.refreshEntries()
			m.inner.SetStatus(i18n.TF("mdviewer.create_ok", values["name"]), false)
		}
	} else {
		path := values["path"]
		target := values["target"]
		if err := m.deleteKnowledgeEntry(path); err != nil {
			m.inner.SetStatus(i18n.TF("mdviewer.delete_failed", err.Error()), true)
		} else {
			m.refreshEntries()
			m.inner.SetStatus(i18n.TF("mdviewer.delete_ok", target), false)
		}
	}
	return m, cmd
}

// createKnowledgeEntry writes a new <agent>/knowledge/<name>/KNOWLEDGE.md
// using the user-entered name verbatim as the folder name. Returns an error
// when the folder already exists or any filesystem op fails.
func (m CodexModel) createKnowledgeEntry(name, description string) error {
	if m.selectedDir == "" {
		return fmt.Errorf("no agent selected")
	}
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return fmt.Errorf("name required")
	}
	folder := filepath.Join(m.selectedDir, "knowledge", name)
	if _, err := os.Stat(folder); err == nil {
		return fmt.Errorf("%s already exists", folder)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	body := buildKnowledgeBody(name, description)
	return os.WriteFile(filepath.Join(folder, "KNOWLEDGE.md"), []byte(body), 0o644)
}

// deleteKnowledgeEntry removes the folder that contains the given
// KNOWLEDGE.md file. Refuses to delete the entire knowledge/ root.
func (m CodexModel) deleteKnowledgeEntry(path string) error {
	if path == "" {
		return fmt.Errorf("missing path")
	}
	dir := filepath.Dir(path)
	root := filepath.Join(m.selectedDir, "knowledge")
	if dir == root || dir == m.selectedDir {
		return fmt.Errorf("refusing to delete knowledge root")
	}
	// Sanity check: dir must be under <agent>/knowledge.
	rel, err := filepath.Rel(root, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %s is outside knowledge root", dir)
	}
	return os.RemoveAll(dir)
}

func (m *CodexModel) refreshEntries() {
	entries := buildAgentCodexEntries(m.selectedDir)
	m.inner.SetEntries(entries)
}

// buildKnowledgeBody composes the initial KNOWLEDGE.md body with a YAML
// frontmatter block plus a stubbed body that matches the parser's
// expectations (cleanFrontmatterScalar / parseFrontmatter).
func buildKnowledgeBody(name, description string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(yamlScalar(name))
	b.WriteString("\n")
	b.WriteString("description: ")
	b.WriteString(yamlScalar(description))
	b.WriteString("\n")
	b.WriteString("---\n\n")
	b.WriteString("# ")
	b.WriteString(name)
	b.WriteString("\n\n")
	b.WriteString(description)
	b.WriteString("\n")
	return b.String()
}

// yamlScalar quotes a value when it contains characters that would otherwise
// confuse a naive YAML reader (colons, leading/trailing whitespace, etc.).
func yamlScalar(s string) string {
	if s == "" {
		return "\"\""
	}
	if strings.ContainsAny(s, ":#\"\n") || strings.TrimSpace(s) != s {
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return s
}

func (m CodexModel) updatePicker(msg tea.KeyPressMsg) (CodexModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+t":
		m.pickerOpen = false
		m.syncPicker()
		return m, nil
	case "up", "k":
		if m.pickerIdx > 0 {
			m.pickerIdx--
			m.syncPicker()
		}
		return m, nil
	case "down", "j":
		if m.pickerIdx < len(m.agentNodes)-1 {
			m.pickerIdx++
			m.syncPicker()
		}
		return m, nil
	case "enter":
		if m.pickerIdx < len(m.agentNodes) {
			newDir := m.agentNodes[m.pickerIdx].WorkingDir
			if newDir != "" && newDir != m.selectedDir {
				m.selectedDir = newDir
				entries := buildAgentCodexEntries(m.selectedDir)
				m.inner = NewMarkdownViewer(entries, codexTitleFor(m.selectedDir))
				m.inner.FooterHint = i18n.T("hints.props_select")
				m.inner.EnableCreate = true
				m.inner.EnableDelete = true
				if m.width > 0 && m.height > 0 {
					var cmd tea.Cmd
					m.inner, cmd = m.inner.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
					m.pickerOpen = false
					m.syncPicker()
					return m, cmd
				}
			}
		}
		m.pickerOpen = false
		m.syncPicker()
		return m, nil
	}
	return m, nil
}

func (m *CodexModel) syncPicker() {
	if !m.ready {
		return
	}
	if m.pickerOpen {
		m.pickerVP.SetContent(m.renderPicker())
	}
}

func (m CodexModel) renderPicker() string {
	sectionStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(ColorText)
	selectedStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "  "+sectionStyle.Render(i18n.T("props.select_agent")))
	lines = append(lines, "")

	if len(m.agentNodes) == 0 {
		lines = append(lines, "  "+StyleFaint.Render("(no agents)"))
		lines = append(lines, "")
		lines = append(lines, "  "+StyleFaint.Render("[esc/ctrl+t] "+i18n.T("manage.back")))
		return strings.Join(lines, "\n")
	}

	for i, n := range m.agentNodes {
		name := n.AgentName
		if n.Nickname != "" {
			name = n.Nickname
		}
		if name == "" {
			name = "(unknown)"
		}

		state := n.State
		if state == "" {
			state = "──"
		}
		stateRendered := lipgloss.NewStyle().Foreground(StateColor(strings.ToUpper(state))).Render(state)

		marker := "  "
		style := nameStyle
		if n.WorkingDir == m.selectedDir {
			marker = "● "
		}
		if i == m.pickerIdx {
			style = selectedStyle
			marker = "> "
			if n.WorkingDir == m.selectedDir {
				marker = ">●"
			}
		}

		lines = append(lines, fmt.Sprintf("  %s%-18s %s", marker, style.Render(name), stateRendered))
	}

	lines = append(lines, "")
	lines = append(lines, "  "+StyleFaint.Render("↑↓ "+i18n.T("manage.select")+"  [enter]  [esc/ctrl+t] "+i18n.T("manage.back")))

	return strings.Join(lines, "\n")
}

func (m CodexModel) View() string {
	if m.pickerOpen {
		header := StyleTitle.Render("  "+codexTitleFor(m.selectedDir)) + "\n" + strings.Repeat("─", m.width)
		footer := strings.Repeat("─", m.width) + "\n" +
			StyleFaint.Render("  "+i18n.T("hints.props_select"))
		body := ""
		if m.ready {
			body = m.pickerVP.View()
		}
		return header + "\n" + PaintViewportBG(body, m.width) + "\n" + footer
	}
	if m.prompt != nil {
		return overlayPrompt(m.inner.View(), m.prompt.View())
	}
	return m.inner.View()
}

package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/anthropics/lingtai-tui/i18n"
)

// entryPromptField is one step in a multi-field inline prompt. Label is the
// localized header shown above the input; Key is the field identifier the
// caller uses when consuming the collected values.
type entryPromptField struct {
	Key         string
	Label       string
	Placeholder string
	Required    bool
	CharLimit   int
}

// entryPromptKind discriminates between create and delete-confirm prompts so
// the renderer can show the right header / styling.
type entryPromptKind int

const (
	entryPromptCreate entryPromptKind = iota
	entryPromptConfirmDelete
)

// entryPrompt is a small inline state machine for collecting one or more
// string fields from the user, plus a y/N confirmation variant for delete.
// It is rendered in the footer area by the parent model.
//
// Usage:
//
//	p := newCreatePrompt("Knowledge", fields)
//	// each Update returns the prompt; check p.Done() and p.Cancelled() after.
//	p, cmd = p.Update(msg)
//	if p.Done() { values := p.Values(); ... }
type entryPrompt struct {
	kind       entryPromptKind
	title      string
	fields     []entryPromptField
	values     map[string]string
	fieldIdx   int
	input      textinput.Model
	confirmYes bool // for entryPromptConfirmDelete

	done      bool
	cancelled bool
	width     int
}

func newCreatePrompt(title string, fields []entryPromptField) entryPrompt {
	p := entryPrompt{
		kind:   entryPromptCreate,
		title:  title,
		fields: fields,
		values: make(map[string]string),
	}
	p.spawnInput()
	return p
}

func newDeleteConfirmPrompt(title string, target string) entryPrompt {
	return entryPrompt{
		kind:   entryPromptConfirmDelete,
		title:  title,
		values: map[string]string{"target": target},
	}
}

func (p *entryPrompt) spawnInput() {
	if p.fieldIdx >= len(p.fields) {
		return
	}
	f := p.fields[p.fieldIdx]
	ti := textinput.New()
	ti.Placeholder = f.Placeholder
	limit := f.CharLimit
	if limit <= 0 {
		limit = 200
	}
	ti.CharLimit = limit
	ti.SetWidth(50)
	ti.Prompt = ""
	ti.Focus()
	p.input = ti
}

func (p entryPrompt) Done() bool      { return p.done }
func (p entryPrompt) Cancelled() bool { return p.cancelled }

// Values returns the collected field values keyed by field Key. For a delete
// confirmation it returns {"confirmed": "yes"} on accept.
func (p entryPrompt) Values() map[string]string {
	out := make(map[string]string, len(p.values))
	for k, v := range p.values {
		out[k] = v
	}
	return out
}

func (p entryPrompt) Update(msg tea.Msg) (entryPrompt, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch p.kind {
	case entryPromptConfirmDelete:
		switch strings.ToLower(key.String()) {
		case "esc", "n":
			p.cancelled = true
			return p, nil
		case "y":
			p.values["confirmed"] = "yes"
			p.done = true
			return p, nil
		case "enter":
			// Enter without an explicit "y" stays safe: treat as cancel.
			p.cancelled = true
			return p, nil
		}
		return p, nil
	case entryPromptCreate:
		switch key.String() {
		case "esc":
			p.cancelled = true
			return p, nil
		case "enter":
			val := strings.TrimSpace(p.input.Value())
			if p.fieldIdx < len(p.fields) {
				f := p.fields[p.fieldIdx]
				if f.Required && val == "" {
					return p, nil // ignore empty submit on a required field
				}
				p.values[f.Key] = val
				p.fieldIdx++
				if p.fieldIdx >= len(p.fields) {
					p.done = true
					return p, nil
				}
				p.spawnInput()
				return p, nil
			}
			p.done = true
			return p, nil
		}
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return p, cmd
	}
	return p, nil
}

// SetWidth updates the inner textinput width so the inline prompt rendering
// matches the available footer width.
func (p *entryPrompt) SetWidth(w int) {
	p.width = w
	if w > 10 {
		p.input.SetWidth(w - 10)
	}
}

// View renders the inline prompt block (header line + input/confirm line).
// Wrappers typically print this in place of mdviewer's standard footer.
func (p entryPrompt) View() string {
	headerStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	switch p.kind {
	case entryPromptConfirmDelete:
		target := p.values["target"]
		prompt := i18n.TF("mdviewer.confirm_delete", target)
		hint := StyleFaint.Render("  [y/N] " + i18n.T("mdviewer.confirm_hint"))
		return "  " + headerStyle.Render(p.title) + "  " + prompt + "\n" + hint
	case entryPromptCreate:
		if p.fieldIdx >= len(p.fields) {
			return ""
		}
		f := p.fields[p.fieldIdx]
		step := i18n.TF("mdviewer.create_step", p.fieldIdx+1, len(p.fields))
		header := headerStyle.Render(p.title) + "  " + StyleFaint.Render(step) + "  " + f.Label
		hint := StyleFaint.Render("  [enter] " + i18n.T("mdviewer.create_next") + "  [esc] " + i18n.T("firstrun.back"))
		return "  " + header + "\n" + "  " + p.input.View() + "\n" + hint
	}
	return ""
}

// PromptLineCount returns the number of lines the prompt occupies in the
// footer, useful when the wrapper needs to budget viewport height.
func (p entryPrompt) PromptLineCount() int {
	switch p.kind {
	case entryPromptConfirmDelete:
		return 2
	case entryPromptCreate:
		return 3
	}
	return 0
}

// overlayPrompt replaces the inner view's footer hint with the prompt block.
// mdviewer reserves 2 footer lines (divider + hint); we drop the hint line so
// the prompt visually attaches to the divider above it.
func overlayPrompt(innerView, promptBlock string) string {
	lines := strings.Split(innerView, "\n")
	if len(lines) >= 1 {
		lines = lines[:len(lines)-1]
	}
	out := strings.Join(lines, "\n")
	return out + "\n" + promptBlock
}

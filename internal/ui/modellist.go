package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dinhlongviolin1/clavis/internal/provider"
)

// Styles for the model selector.
var (
	msCursor   = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	msSelected = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	msNormal   = lipgloss.NewStyle()
	msOwner    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	msCount    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	msNoMatch  = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
)

// selectItem is one entry in the model list.
type selectItem struct {
	label string // model ID
	owner string // provider name (dim)
	value string // returned on select
}

// modelSelect is a bubbletea model for interactive model selection.
// Unlike huh.Select, it resets the cursor to the top on every filter
// change and shows ↑/↓ position counts.
type modelSelect struct {
	title    string
	hint     string
	items    []selectItem // all items
	filtered []selectItem // current filter results
	cursor   int          // index into filtered
	offset   int          // first visible row
	height   int          // visible rows
	input    textinput.Model
	aborted  bool
}

func newModelSelect(title, hint string, items []selectItem, height int) modelSelect {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 80
	ti.Focus()

	m := modelSelect{
		title:  title,
		hint:   hint,
		items:  items,
		height: height,
		input:  ti,
	}
	m.applyFilter()
	return m
}

// applyFilter re-filters items and resets cursor to top.
func (m *modelSelect) applyFilter() {
	q := strings.ToLower(m.input.Value())
	if q == "" {
		m.filtered = m.items
	} else {
		m.filtered = nil
		for _, item := range m.items {
			if strings.Contains(strings.ToLower(item.label), q) ||
				strings.Contains(strings.ToLower(item.owner), q) {
				m.filtered = append(m.filtered, item)
			}
		}
	}
	m.cursor = 0
	m.offset = 0
}

func (m modelSelect) Init() tea.Cmd {
	return textinput.Blink
}

func (m modelSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.aborted = true
			return m, tea.Quit
		case "enter":
			if len(m.filtered) > 0 {
				return m, tea.Quit
			}
			return m, nil
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
			return m, nil
		case "down", "ctrl+n":
			if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
				m.cursor++
				if m.cursor >= m.offset+m.height {
					m.offset = m.cursor - m.height + 1
				}
			}
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	// Forward to text input for live search.
	var cmd tea.Cmd
	prev := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prev {
		m.applyFilter()
	}
	return m, cmd
}

func (m modelSelect) View() string {
	var sb strings.Builder

	total := len(m.items)
	matched := len(m.filtered)
	above := m.cursor
	below := 0
	if matched > 0 {
		below = matched - m.cursor - 1
	}

	// Title + counts: "Opus model  3/20  ↑1 ↓1"
	counts := msCount.Render(fmt.Sprintf("%d/%d", matched, total))
	pos := msCount.Render(fmt.Sprintf("↑%d ↓%d", above, below))
	sb.WriteString(TitleStyle.Render(m.title) + "  " + counts + "  " + pos + "\n")

	// Search input
	sb.WriteString(m.input.View() + "\n")

	// Model list (height rows)
	if matched == 0 {
		sb.WriteString(msNoMatch.Render("  no matches"))
	} else {
		end := m.offset + m.height
		if end > matched {
			end = matched
		}
		for i := m.offset; i < end; i++ {
			item := m.filtered[i]
			label := item.label
			if item.owner != "" {
				label += "  " + msOwner.Render("("+item.owner+")")
			}
			if i == m.cursor {
				sb.WriteString(msCursor.Render("> ") + msSelected.Render(label))
			} else {
				sb.WriteString("  " + msNormal.Render(label))
			}
			if i < end-1 {
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("\n" + DimStyle.Render(m.hint))
	return sb.String()
}

// RunModelSelect shows an interactive model selector.
// extraLabel adds a final option (e.g. "(same as opus)") with empty value.
// Returns the selected value, or errBack on Esc.
func RunModelSelect(title string, models []provider.ModelInfo, height int, extraLabel string) (string, error) {
	items := make([]selectItem, 0, len(models)+1)
	for _, m := range models {
		items = append(items, selectItem{label: m.ID, owner: m.OwnedBy, value: m.ID})
	}
	if extraLabel != "" {
		items = append(items, selectItem{label: extraLabel, value: ""})
	}

	hint := "↑↓ navigate · enter select · esc back · type to filter"
	prog := tea.NewProgram(newModelSelect(title, hint, items, height))
	final, err := prog.Run()
	if err != nil {
		return "", err
	}
	result := final.(modelSelect)
	if result.aborted || len(result.filtered) == 0 {
		return "", errBack
	}
	return result.filtered[result.cursor].value, nil
}

package ui

import (
	"fmt"
	"os"
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
	msPin      = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // yellow star
)

// selectItem is one entry in the model list.
type selectItem struct {
	label  string // model ID
	owner  string // provider name (dim)
	value  string // returned on select
	pinned bool   // recently chosen — floated to the top with a marker
}

// modelSelect is a bubbletea model for interactive model selection.
// Unlike huh.Select, it resets the cursor to the top on every filter
// change, shows ↑/↓ position counts, and floats recently-chosen models
// to the top of the list.
type modelSelect struct {
	title       string
	hint        string
	items       []selectItem // all items (pinned first)
	filtered    []selectItem // current filter results
	cursor      int          // index into filtered
	offset      int          // first visible row
	height      int          // visible rows
	input       textinput.Model
	aborted     bool // esc — caller treats as "go back"
	interrupted bool // ctrl+c — caller exits the process
}

func newModelSelect(title, hint string, items []selectItem, height int) modelSelect {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 80
	// Width must be >= the placeholder length, otherwise bubbles' textinput
	// sizes its placeholder buffer to Width+1 runes and truncates it (with the
	// default Width of 0 the placeholder collapses to a single "t").
	ti.Width = 40
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

// applyFilter re-filters items and resets the cursor to the top.
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
		case "ctrl+c":
			m.interrupted = true
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
		}
	}

	// Forward everything else to the text input for live search.
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

	// Title line: "Opus model   3/20   ↑1 ↓1"
	counts := msCount.Render(fmt.Sprintf("%d/%d", matched, total))
	pos := msCount.Render(fmt.Sprintf("↑%d ↓%d", above, below))
	sb.WriteString(TitleStyle.Render(m.title) + "  " + counts + "  " + pos + "\n")

	// Search input.
	sb.WriteString(m.input.View() + "\n")

	// Model list (up to `height` rows).
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
			if item.pinned {
				label = msPin.Render("★ ") + label
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

// buildSelectItems produces the ordered list for the selector: the optional
// extra entry first (e.g. "(default — use sonnet)", so the fallback is the
// default the cursor lands on), then pinned models (deduped, in the given
// order, only if present in the catalog), then the remaining catalog models.
func buildSelectItems(models []provider.ModelInfo, extraLabel string, pinned []string) []selectItem {
	pinnedSet := make(map[string]bool, len(pinned))
	owner := make(map[string]string, len(models))
	for _, m := range models {
		owner[m.ID] = m.OwnedBy
	}

	items := make([]selectItem, 0, len(models)+1)
	if extraLabel != "" {
		items = append(items, selectItem{label: extraLabel, value: ""})
	}
	for _, id := range pinned {
		if id == "" || pinnedSet[id] {
			continue
		}
		if _, ok := owner[id]; !ok {
			continue // pinned model not in this catalog — skip
		}
		pinnedSet[id] = true
		items = append(items, selectItem{label: id, owner: owner[id], value: id, pinned: true})
	}
	for _, m := range models {
		if pinnedSet[m.ID] {
			continue
		}
		items = append(items, selectItem{label: m.ID, owner: m.OwnedBy, value: m.ID})
	}
	return items
}

// RunModelSelect shows an interactive model selector on a cleared screen.
//
//   - extraLabel: optional entry shown first (e.g. "(default — use sonnet)")
//     with an empty return value — the cursor starts on it so it's the default.
//   - pinned: model IDs already chosen for earlier tiers. They are floated below
//     the extra entry with a ★ marker, so a just-picked model stays one keypress
//     away.
//
// Returns the chosen value, or errBack on Esc. Ctrl+C exits the process (130).
func RunModelSelect(title string, models []provider.ModelInfo, height int, extraLabel string, pinned []string) (string, error) {
	items := buildSelectItems(models, extraLabel, pinned)
	hint := "↑↓ navigate · enter select · esc back · type to filter"

	// Clear the screen so each selection is its own clean view.
	clearScreen()
	prog := tea.NewProgram(newModelSelect(title, hint, items, height))
	final, err := prog.Run()
	if err != nil {
		return "", err
	}
	result := final.(modelSelect)
	if result.interrupted {
		os.Exit(130)
	}
	if result.aborted || len(result.filtered) == 0 {
		return "", errBack
	}
	return result.filtered[result.cursor].value, nil
}

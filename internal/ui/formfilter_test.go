package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func TestFormFilter(t *testing.T) {
	var interrupted bool
	filter := formFilter(&interrupted)

	// Below the minimum the width is raised; at or above it nothing changes.
	for _, tc := range []struct{ in, want int }{{0, minFormWidth}, {3, minFormWidth}, {minFormWidth, minFormWidth}, {40, 40}} {
		msg := filter(nil, tea.WindowSizeMsg{Width: tc.in, Height: 40})
		ws, ok := msg.(tea.WindowSizeMsg)
		if !ok {
			t.Fatalf("width %d: filter returned %T, want tea.WindowSizeMsg", tc.in, msg)
		}
		if ws.Width != tc.want {
			t.Errorf("width %d: got %d, want %d", tc.in, ws.Width, tc.want)
		}
		if ws.Height != 40 {
			t.Errorf("width %d: height became %d; only the width should be clamped", tc.in, ws.Height)
		}
	}

	if interrupted {
		t.Error("a window-size message set the ctrl+c flag")
	}
	if filter(nil, tea.KeyMsg{Type: tea.KeyCtrlC}); !interrupted {
		t.Error("ctrl+c was not flagged")
	}
}

// Regression test for the crash the clamp exists for: huh's layout maths hands
// bubbles' textinput a width of -2 or less, and `make([]rune, m.Width+1)`
// panics. Fed through the filter, the same sizes render instead.
func TestNarrowTerminalDoesNotPanic(t *testing.T) {
	var a, b, c, d string

	var interrupted bool
	filter := formFilter(&interrupted)

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Opus model ID").Placeholder("model-id").Value(&a),
		huh.NewInput().Title("Sonnet model ID").Placeholder("same as opus").Value(&b),
		huh.NewInput().Title("Haiku model ID").Placeholder("same as opus").Value(&c),
		huh.NewInput().Title("Subagent model ID").Placeholder("same as opus").Value(&d),
	))
	_ = form.Init()

	for _, w := range []int{0, 1, 3, 4, 8, 40} {
		form.Update(filter(nil, tea.WindowSizeMsg{Width: w, Height: 40}))
		_ = form.View()
	}
}

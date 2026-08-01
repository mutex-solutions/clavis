package ui

import (
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// fetchDoneMsg carries the result of a background fetch back to the spinner.
type fetchDoneMsg struct {
	result any
	err    error
}

// loadingModel shows an animated spinner while a blocking task runs, so the
// screen never sits empty during a network fetch. It quits as soon as the task
// signals completion (or the user presses Ctrl+C).
type loadingModel struct {
	spinner     spinner.Model
	label       string
	result      any
	err         error
	done        bool
	interrupted bool
}

func newLoadingModel(label string) loadingModel {
	s := spinner.New(spinner.WithSpinner(spinner.Dot))
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	return loadingModel{spinner: s, label: label}
}

func (m loadingModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.interrupted = true
			return m, tea.Quit
		}
		return m, nil
	case fetchDoneMsg:
		m.result = msg.result
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
}

func (m loadingModel) View() string {
	if m.done {
		return ""
	}
	return m.spinner.View() + " " + TitleStyle.Render(m.label)
}

// Spinner runs `task` on a background goroutine while showing an animated
// spinner labelled `label`, then returns the task's error. Use it to keep the
// screen alive during a blocking call (e.g. a network ping). Ctrl+C during the
// wait exits the process (130).
func Spinner(label string, task func() error) error {
	_, err := runSpinner(label, false, func() (any, error) {
		return nil, task()
	})
	return err
}

// runWithSpinner shows an animated spinner labelled `label` while `task` runs on
// a background goroutine, and returns whatever the task produced. Ctrl+C during
// the wait exits the process (130); the spinner clears itself on completion.
func runWithSpinner(label string, task func() (any, error)) (any, error) {
	return runSpinner(label, true, task)
}

func runSpinner(label string, clear bool, task func() (any, error)) (any, error) {
	if clear {
		clearScreen()
	}

	prog := tea.NewProgram(newLoadingModel(label))
	go func() {
		res, err := task()
		prog.Send(fetchDoneMsg{result: res, err: err})
	}()

	final, runErr := prog.Run()
	if runErr != nil {
		// The tea program itself failed — fall back to running the task
		// synchronously so the wizard can still proceed.
		return task()
	}
	lm := final.(loadingModel)
	if lm.interrupted {
		os.Exit(130)
	}
	return lm.result, lm.err
}

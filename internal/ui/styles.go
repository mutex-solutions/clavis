package ui

import "github.com/charmbracelet/lipgloss"

// Styles for terminal output.
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	TierStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	CommandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	WarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))
)

// Success returns a green checkmark message.
func Success(msg string) string {
	return SuccessStyle.Render("✓ " + msg)
}

// Error returns a red X message.
func Error(msg string) string {
	return ErrorStyle.Render("✗ " + msg)
}

// Warn returns a yellow warning message.
func Warn(msg string) string {
	return WarnStyle.Render("⚠ " + msg)
}

// Info returns a dim informational message.
func Info(msg string) string {
	return DimStyle.Render("  " + msg)
}

// NextStep tells the user what command to run next.
func NextStep(prefix, cmd string) string {
	return DimStyle.Render("  " + prefix + " ") + CommandStyle.Render(cmd)
}

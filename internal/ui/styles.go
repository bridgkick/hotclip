package ui

import "charm.land/lipgloss/v2"

var (
	orange = lipgloss.Color("#FF8C00")
	amber  = lipgloss.Color("#FFBF00")
	dim    = lipgloss.Color("#666666")

	titleStyle = lipgloss.NewStyle().
			Foreground(orange).
			Bold(true)

	badgeStyle = lipgloss.NewStyle().
			Foreground(amber)

	dimStyle = lipgloss.NewStyle().
			Foreground(dim)

	selectedStyle = lipgloss.NewStyle().
			Foreground(orange).
			Bold(true)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(orange).
			Bold(true).
			PaddingLeft(2)

	formFieldStyle = lipgloss.NewStyle().
			PaddingLeft(4)
)

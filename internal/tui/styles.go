package tui

import "github.com/charmbracelet/lipgloss"

var (
	Yellow  = lipgloss.Color("226")
	Black   = lipgloss.Color("016")
	Gray    = lipgloss.Color("248")
	Blue    = lipgloss.Color("068")
	Green   = lipgloss.Color("114")
	Magenta = lipgloss.Color("205")

	Household = lipgloss.NewStyle().Bold(true).Underline(true).PaddingTop(2)
	Dead      = lipgloss.NewStyle().Foreground(Gray)
	// Address     = lipgloss.NewStyle().Foreground(Blue).BorderStyle(lipgloss.RoundedBorder())
	Address     = lipgloss.NewStyle().Foreground(Blue)
	Date        = lipgloss.NewStyle().Foreground(Green)
	Anniversary = lipgloss.NewStyle().Foreground(Magenta).SetString("Anniversary: ")
	Indent      = lipgloss.NewStyle().PaddingLeft(2)

	Generation = map[int]lipgloss.Style{
		0: lipgloss.NewStyle().Bold(true).Underline(true),
		1: lipgloss.NewStyle().Bold(true).PaddingLeft(4),
		2: lipgloss.NewStyle().Bold(true),
	}
)

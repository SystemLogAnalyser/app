package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/coreos/go-systemd/sdjournal"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF7CCB")).
			Padding(0, 1).
			AlignHorizontal(lipgloss.Center).Border(lipgloss.NormalBorder())
	tabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777")).
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("#7D5674")).
			Padding(0, 1)
	activeTabStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(lipgloss.Color("#7D5674")).
			Padding(0, 1)
	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(1, 2)
	helpStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#444444")).
			Foreground(lipgloss.Color("#FFFFFF"))
	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("#444444")).
			Foreground(lipgloss.Color("#00FF00"))
	helpSeparatorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#444444")).
				Foreground(lipgloss.Color("#888888"))
)

const (
	Errors = iota
	Warnings
	Information
	Misc
	All
)

type focusedInput int

const (
	logFocus focusedInput = iota
	searchBoxFocused
	startDateFocused
	endDateFocused
)

type model struct {
	width        int
	height       int
	activeTab    int
	focused      focusedInput
	searchBox    textinput.Model
	startDate    textinput.Model
	endDate      textinput.Model
	searchQuery  string
	errors       []Log
	warnings     []Log
	info         []Log
	misc         []Log
	filteredLogs []Log
	logTable     table.Model
	chart        barchart.Model
}

func (m *model) Init() tea.Cmd {
	// Initialize tables

	// m.chart = barchart.New(11, 10)
	m.chart.Draw()
	m.initLogTable()
	// m.initHelpTable()
	return tea.EnterAltScreen
}

func (m *model) initLogTable() {
	columns := []table.Column{
		{Title: "Timestamp", Width: 20},
		{Title: "Process", Width: 20},
		{Title: "Message", Width: 80}, // Remaining width for message
	}

	// Convert filtered logs to table rows
	rows := make([]table.Row, len(m.filteredLogs))
	for i, log := range m.filteredLogs {
		rows[i] = table.Row{log.timestamp, log.process, log.message}
	}

	m.logTable = table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(20),
		table.WithFocused(m.focused == logFocus),
	)
}

func (m model) renderHelpFooter() string {
	var help strings.Builder

	// Define help items
	helpItems := []struct {
		key         string
		description string
	}{
		{"q", "Exit"},
		{"Tab", "Next Tab"},
		{"Shift-Tab", "Previous Tab"},
		{"/", "Search"},
		{"F", "Start Date"},
		{"E", "End Date"},
		{"Esc", "Clear current filter"},
		{"Enter", "Apply"},
	}

	separator := helpSeparatorStyle.Render(" | ")

	// Create the help line
	columnWidth := 15
	width := 0
	help.WriteString(helpStyle.Render("  "))
	for i, item := range helpItems {
		if i > 0 {
			help.WriteString(separator)
		}
		if width > m.width {
			help.WriteString("\n")
		}
		help.WriteString(helpKeyStyle.Render(item.key))
		help.WriteString(helpStyle.Render(" " + item.description))
		width += columnWidth
	}

	// Create border line
	width = m.width
	if width == 0 {
		width = 80 // fallback width
	}

	// Pad the help text to full width
	helpText := help.String()
	padding := width - lipgloss.Width(helpText)
	if padding > 0 {
		helpText += helpStyle.Render(strings.Repeat(" ", padding))
	}

	return helpText
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focused {
	case searchBoxFocused:
		m.searchBox, cmd = m.searchBox.Update(msg)
	case startDateFocused:
		m.startDate, cmd = m.startDate.Update(msg)
	case endDateFocused:
		m.endDate, cmd = m.endDate.Update(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.focused == logFocus {
				return m, tea.Quit
			}
		case "tab":
			m.activeTab = (m.activeTab + 1) % 5
			m.applyFilters() // Update filtered logs for new tab
			m.initLogTable() // Reinitialize table with new data
		case "shift+tab":
			m.activeTab = (m.activeTab + 5 - 1) % 5
			m.applyFilters() // Update fi5tered logs for new tab
			m.initLogTable() // Reinitialize table with new data
		case "/":
			if m.focused == logFocus {
				m.focused = searchBoxFocused
				m.searchBox.Focus()
				m.startDate.Blur()
				m.endDate.Blur()
			}
		case "f":
			if m.focused == logFocus {
				m.focused = startDateFocused
				m.startDate.Focus()
				m.searchBox.Blur()
				m.endDate.Blur()
			}
		case "e":
			if m.focused == logFocus {
				m.focused = endDateFocused
				m.endDate.Focus()
				m.searchBox.Blur()
				m.startDate.Blur()
			}
		case "esc":
			m.clearFocusedFilter()
			m.focused = logFocus
			m.searchBox.Blur()
			m.startDate.Blur()
			m.endDate.Blur()
			m.initLogTable() // Reinitialize table after clearing filter
		case "enter":
			if m.focused == searchBoxFocused || m.focused == startDateFocused || m.focused == endDateFocused {
				m.applyFilters()
				m.initLogTable() // Reinitialize table after applying filters
				m.focused = logFocus
			}
		}
		// Handle table navigation when focused on logs
		if m.focused == logFocus {
			var tableMsg tea.Msg = msg
			m.logTable, cmd = m.logTable.Update(tableMsg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.initLogTable() // Reinitialize table with new dimensions
		return m, tea.ClearScreen
	}

	return m, cmd
}

func (m model) View() string {
	content := strings.Builder{}

	// Title
	title := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, titleStyle.Render("System Log Analyzer"))
	content.WriteString(title + "\n\n")

	// Tab bar
	var tabBar string
	switch m.activeTab {
	case Errors:
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top,
			activeTabStyle.Render("Errors"),
			tabStyle.Render("Warnings"),
			tabStyle.Render("Information"),
			tabStyle.Render("Misc"),
			tabStyle.Render("All"))
	case Warnings:
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top,
			tabStyle.Render("Errors"),
			activeTabStyle.Render("Warnings"),
			tabStyle.Render("Information"),
			tabStyle.Render("Misc"),
			tabStyle.Render("All"))
	case Information:
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top,
			tabStyle.Render("Errors"),
			tabStyle.Render("Warnings"),
			activeTabStyle.Render("Information"),
			tabStyle.Render("Misc"),
			tabStyle.Render("All"))
	case Misc:
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top,
			tabStyle.Render("Errors"),
			tabStyle.Render("Warnings"),
			tabStyle.Render("Information"),
			activeTabStyle.Render("Misc"),
			tabStyle.Render("All"))
	case All:
		tabBar = lipgloss.JoinHorizontal(lipgloss.Top,
			tabStyle.Render("Errors"),
			tabStyle.Render("Warnings"),
			tabStyle.Render("Information"),
			tabStyle.Render("Misc"),
			activeTabStyle.Render("All"))
	}
	tabBar = lipgloss.JoinHorizontal(lipgloss.Center, tabBar, " ("+strconv.Itoa(len(m.filteredLogs))+")")
	content.WriteString(tabBar + "\n\n")

	// Search and date filters
	content.WriteString("Search: " + m.searchBox.View() + "\n\n")
	content.WriteString("Start Date (YYYY-MM-DD): " + m.startDate.View() + "\n")
	content.WriteString("End Date (YYYY-MM-DD): " + m.endDate.View() + "\n\n")

	// Log table
	border := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	table := border.Render(m.logTable.View())
	data := lipgloss.JoinHorizontal(lipgloss.Top, table, border.Margin(0, 0, 0, 10).Padding(1).Render(m.chart.View()))
	content.WriteString(data)
	// content.WriteString("\nLogs:\n")
	// content.WriteString(m.logTable.View())
	//
	// // m.chart.Draw()
	// content.WriteString("\n\nChart:\n")
	// content.WriteString(m.chart.View())

	// Help table
	footer := lipgloss.PlaceVertical(m.height-lipgloss.Height(content.String()), lipgloss.Bottom, m.renderHelpFooter())
	content.WriteString(footer)
	contentStyle := lipgloss.NewStyle().MarginLeft(1)

	return contentStyle.Render(content.String())
}

func filterLogs(logs []Log, query, start, end string) []Log {
	var result []Log
	for _, log := range logs {
		if query != "" && !strings.Contains(strings.ToLower(log.message), strings.ToLower(query)) {
			continue
		}
		if start != "" && log.timestamp < start {
			continue
		}
		if end != "" && log.timestamp > end {
			continue
		}
		result = append(result, log)
	}
	return result
}

func (m *model) clearFocusedFilter() {
	switch m.focused {
	case searchBoxFocused:
		m.searchBox.SetValue("")
	case startDateFocused:
		m.startDate.SetValue("")
	case endDateFocused:
		m.endDate.SetValue("")
	}
	m.applyFilters()
}

func (m *model) applyFilters() {
	var logs []Log
	switch m.activeTab {
	case Errors:
		logs = m.errors
	case Warnings:
		logs = m.warnings
	case Information:
		logs = m.info
	case Misc:
		logs = m.misc
	case All:
		var all []Log
		all = append(all, m.errors...)
		all = append(all, m.warnings...)
		all = append(all, m.info...)
		all = append(all, m.misc...)
		logs = all
	}
	m.filteredLogs = filterLogs(logs, m.searchBox.Value(), m.startDate.Value(), m.endDate.Value())
}

func customFormatter(entry *sdjournal.JournalEntry) (string, error) {
	msg, ok := entry.Fields["SYSLOG_RAW"]
	if !ok {
		return "", fmt.Errorf("no MESSAGE field present in journal entry")
	}
	hostname, _ := entry.Fields["_HOSTNAME"]
	process, _ := entry.Fields["_COMM"]

	usec := entry.RealtimeTimestamp
	timestamp := time.Unix(0, int64(usec)*int64(time.Microsecond)).Format("Jan 02 15:04:05")

	return fmt.Sprintf("%s %s %s: %s\n", timestamp, hostname, process, msg), nil
}

func main() {
	searchBox := textinput.New()
	searchBox.Placeholder = "Enter keyword"
	searchBox.Width = 30

	startDate := textinput.New()
	startDate.Placeholder = "YYYY-MM-DD"
	startDate.Width = 12

	endDate := textinput.New()
	endDate.Placeholder = "YYYY-MM-DD"
	endDate.Width = 12

	var readers []io.Reader
	for _, filePath := range os.Args[1:] {
		logFile, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("[ERROR] Could not open file %s: %s", filePath, err)
		}
		defer logFile.Close()
		readers = append(readers, logFile)
	}
	if len(readers) == 0 {
		// journalReader, err := sdjournal.NewJournalReader(sdjournal.JournalReaderConfig{
		// 	Formatter: customFormatter,
		// })
		// if err != nil {
		// 	fmt.Println("No valid file paths provided and could not read system journal")
		// }
		// readers = append(readers, journalReader)
		fmt.Println("No valid file paths provided")
		os.Exit(0)
	}

	multiReader := io.MultiReader(readers...)
	logs := GetCategorisedLogs(multiReader)

	d1 := barchart.BarData{
		Label: "Error",
		Values: []barchart.BarValue{
			{
				Name:  "Errors",
				Value: float64(len(logs["errors"])),
				Style: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			},
		},
	}
	d2 := barchart.BarData{
		Label: "Warn",
		Values: []barchart.BarValue{
			{
				Value: float64(len(logs["warnings"])),
				Style: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			},
		},
	}
	d3 := barchart.BarData{
		Label: "Info",
		Values: []barchart.BarValue{
			{
				Value: float64(len(logs["infos"])),
				Style: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			},
		},
	}
	d4 := barchart.BarData{
		Label: "Misc",
		Values: []barchart.BarValue{
			{
				Value: float64(len(logs["misc"])),
				Style: lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
			},
		},
	}
	d5 := barchart.BarData{
		Label: "All",
		Values: []barchart.BarValue{
			{
				Value: float64(len(logs["info"]) + len(logs["warnings"]) + len(logs["errors"]) + len(logs["misc"])),
				Style: lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
			},
		},
	}

	m := model{
		searchBox: searchBox,
		startDate: startDate,
		endDate:   endDate,
		errors:    logs["errors"],
		warnings:  logs["warnings"],
		info:      logs["infos"],
		misc:      logs["misc"],
		chart: barchart.New(
			32,
			18,
			barchart.WithDataSet([]barchart.BarData{d1, d2, d3, d4, d5}),
		),
	}

	m.applyFilters() // Initialize filtered logs

	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}
}

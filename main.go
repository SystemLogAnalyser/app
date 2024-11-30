package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/charmbracelet/bubbles/textinput"
	// "github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/coreos/go-systemd/sdjournal"
)

type Log struct {
	timestamp time.Time
	process   string
	message   string
	category  string
}

type Filter struct {
	name      string
	startTime string
	endTime   string
	process   []*regexp.Regexp
	message   []*regexp.Regexp
	category  string
}

type Tab struct {
	name    string
	filters []Filter
	logs    []Log
}

type model struct {
	width                   int
	height                  int
	tabs                    []Tab
	activeTab               int
	keymaps                 map[string]keymap
	countChart              barchart.Model
	top_tab_processes_chart barchart.Model
	tab_trend_chart         barchart.Model
	table                   table.Model
	logs                    []Log
	msgSearch               textinput.Model
	startDate               textinput.Model
	endDate                 textinput.Model
	processSearch           textinput.Model
	focused                 int
}

const (
	msgSearch = iota
	startDate
	endDate
	processSearch
	logTable
)

type keymap struct {
	callback func(m *model) tea.Cmd
	desc     string
}

func (m *model) Init() tea.Cmd {
	m.countChart.Draw()
	m.top_tab_processes_chart.Draw()
	m.tab_trend_chart.Draw()
	// m.tab_trend_chart.DrawXYAxisAndLabel()
	return tea.EnterAltScreen
}

func (m *model) clearFocused() {
	switch m.focused {
	case msgSearch:
		m.msgSearch.SetValue("")
	case startDate:
		m.startDate.SetValue("")
	case endDate:
		m.endDate.SetValue("")
	case processSearch:
		m.processSearch.SetValue("")
	}
}

func (m *model) updateFocued(msg tea.Msg) (*model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focused {
	case msgSearch:
		m.msgSearch, cmd = m.msgSearch.Update(msg)
	case startDate:
		m.startDate, cmd = m.startDate.Update(msg)
	case endDate:
		m.endDate, cmd = m.endDate.Update(msg)
	case processSearch:
		m.processSearch, cmd = m.processSearch.Update(msg)
	}
	return m, cmd
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.focused == msgSearch || m.focused == startDate || m.focused == endDate || m.focused == processSearch {
			if msg.String() == "enter" {
				m.updateTab()
				m.focused = logTable
				m.msgSearch.Blur()
				m.startDate.Blur()
				m.endDate.Blur()
				m.processSearch.Blur()
			} else if msg.String() == "esc" {
				m.clearFocused()
				m.focused = logTable
				m.msgSearch.Blur()
				m.startDate.Blur()
				m.endDate.Blur()
				m.processSearch.Blur()
				m.updateTab()
			} else {
				m, cmd = m.updateFocued(msg)
			}
			return m, cmd
		} else if keymap, ok := m.keymaps[msg.String()]; ok {
			cmd = keymap.callback(m)
		} else {
			m.table, cmd = m.table.Update(msg)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		cmd = tea.ClearScreen
	}
	return m, cmd
}

func (m model) View() string {
	content := strings.Builder{}
	// header
	content.WriteString(
		lipgloss.PlaceHorizontal(
			m.width,
			lipgloss.Center,
			lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("111")).
				Padding(0, 1).
				AlignHorizontal(lipgloss.Center).Border(lipgloss.NormalBorder()).
				Render("System Log Analyzer"),
		),
	)
	content.WriteString("\n\n")
	// inputs
	content.WriteString("Search: " + m.msgSearch.View() + "\n\n")
	content.WriteString("StartDate: " + m.startDate.View() + "\n\n")
	content.WriteString("EndDate: " + m.endDate.View() + "\n\n")
	content.WriteString("Process: " + m.processSearch.View() + "\n\n")
	// tab bar
	var tabBtns []string
	tabStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		BorderForeground(lipgloss.Color("11"))
	for i, tab := range m.tabs {
		if m.activeTab == i {
			tabStyle = tabStyle.Foreground(lipgloss.Color("15"))
		} else {
			tabStyle = tabStyle.Foreground(lipgloss.Color("8"))
		}
		tabBtns = append(tabBtns, tabStyle.Render(tab.name+" ("+strconv.Itoa(len(tab.logs))+")"))
	}
	content.WriteString(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			tabBtns...,
		),
	)
	content.WriteString("\n\n")

	// table and countChart
	border := lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	content.WriteString(
		lipgloss.JoinHorizontal(lipgloss.Top,
			border.Render(m.table.View()),
			border.Render(m.countChart.View()),
		),
	)
	content.WriteString("\n\n")

	// top_tab_processes_chart and tab_trend_chart
	content.WriteString(
		lipgloss.JoinHorizontal(lipgloss.Top,
			border.Render(m.top_tab_processes_chart.View()),
			border.Render(m.tab_trend_chart.View()),
		),
	)
	content.WriteString("\n\n")

	return content.String()
}

func getCategoryColor(cat string) lipgloss.Color {
	switch cat {
	case "Errors":
		return lipgloss.Color("9")
	case "Warnings":
		return lipgloss.Color("3")
	case "Informational":
		return lipgloss.Color("2")
	case "Uncategorised":
		return lipgloss.Color("4")
	}
	return lipgloss.Color("15")
}

func (m *model) updateTab() {
	filter := Filter {
		name: "temp",
		message: []*regexp.Regexp {
			regexp.MustCompile(m.msgSearch.Value()),
		},
		startTime: m.startDate.Value(),
		endTime: m.endDate.Value(),
		process: []*regexp.Regexp {
			regexp.MustCompile(m.processSearch.Value()),
		},
	}
	filteredLogs := getFilteredLogs(m.tabs[m.activeTab].logs, []Filter {filter})
	var tableRows []table.Row
	for _, log := range filteredLogs {
		tableRows = append(tableRows, table.Row{
			log.timestamp.Format("Jan 02 15:04:05"),
			log.process,
			log.message,
		})
	}
	m.table.SetRows(tableRows)

	m.top_tab_processes_chart.Clear()
	m.tab_trend_chart.Clear()

	if len(filteredLogs) == 0 {
		return
	}
	var topProcessChartData []barchart.BarData
	for i, process := range getTopNProcess(filteredLogs, 5) {
		topProcessChartData = append(topProcessChartData, barchart.BarData{
			Label: process.s,
			Values: []barchart.BarValue{
				{
					Value: float64(process.count),
					Style: lipgloss.NewStyle().Foreground(lipgloss.Color(strconv.Itoa(i + 1))),
				},
			},
		})
	}
	m.top_tab_processes_chart.PushAll(topProcessChartData)
	m.top_tab_processes_chart.Draw()

	var trendChartData []barchart.BarData
	for _, trend := range getTrendValues(filteredLogs, 5) {
		trendChartData = append(trendChartData, barchart.BarData{
			// Label: "",
			Values: []barchart.BarValue{
				{
					Value: float64(trend),
					Style: lipgloss.NewStyle().Foreground(lipgloss.Color("15")),
				},
			},
		})
	}
	m.tab_trend_chart.PushAll(trendChartData)
	m.tab_trend_chart.Draw()
}

func main() {
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
		journalReader, err := sdjournal.NewJournalReader(sdjournal.JournalReaderConfig{
			Formatter: func(entry *sdjournal.JournalEntry) (string, error) {
				msg, ok := entry.Fields["MESSAGE"]
				if !ok {
					return "", fmt.Errorf("no MESSAGE field present in journal entry")
				}
				hostname, _ := entry.Fields["_HOSTNAME"]
				process, _ := entry.Fields["_COMM"]

				usec := entry.RealtimeTimestamp
				timestamp := time.Unix(0, int64(usec)*int64(time.Microsecond)).Format("Jan 02 15:04:05")

				return fmt.Sprintf("%s %s %s: %s\n", timestamp, hostname, process, msg), nil
			},
		})
		if err != nil {
			fmt.Println("No valid file paths provided and could not read system journal")
		}
		readers = append(readers, journalReader)
	}

	multiReader := io.MultiReader(readers...)

	parsedLogs := GetParsedLogs(multiReader)
	logs := GetCategorisedLogs(parsedLogs)

	categoryFilters := []Filter{{
		name:     "Errors",
		category: "error",
	}, {
		name:     "Warnings",
		category: "warn",
	}, {
		name:     "Informational",
		category: "info",
	}, {
		name:     "Uncategorised",
		category: "misc",
	}, {
		name: "All",
	}}

	var tabs []Tab
	var countChartData []barchart.BarData
	for _, filter := range categoryFilters {
		tabLogs := getFilteredLogs(logs, []Filter{filter})
		tabs = append(tabs, Tab{
			name:    filter.name,
			filters: []Filter{filter},
			logs:    tabLogs,
		})
		countChartData = append(countChartData, barchart.BarData{
			Label: filter.name,
			Values: []barchart.BarValue{
				{
					Value: float64(len(tabLogs)),
					Style: lipgloss.NewStyle().Foreground(getCategoryColor(filter.name)),
				},
			},
		})
	}

	msgSearchBox := textinput.New()
	msgSearchBox.Width = 30
	startDateBox := textinput.New()
	startDateBox.Width = 30
	endDateBox := textinput.New()
	endDateBox.Width = 30
	processSearchBox := textinput.New()
	processSearchBox.Width = 30

	m := model{
		activeTab:  0,
		focused: logTable,
		msgSearch: msgSearchBox,
		startDate: startDateBox,
		endDate: endDateBox,
		processSearch: processSearchBox,
		countChart: barchart.New(64, 20, barchart.WithDataSet(countChartData)),
		top_tab_processes_chart: barchart.New(
			64,
			16,
			barchart.WithHorizontalBars(),
		),
		tab_trend_chart: barchart.New(64, 16, barchart.WithNoAxis()),
		table: table.New(
			table.WithHeight(20),
			table.WithColumns([]table.Column{
				{Title: "Timestamp", Width: 16},
				{Title: "Process", Width: 20},
				{Title: "Message", Width: 80},
			})),
		logs: logs,
		tabs: tabs,
		keymaps: map[string]keymap{
			"q": {
				desc: "quit",
				callback: func(m *model) tea.Cmd {
					return tea.Quit
				},
			},
			"tab": {
				desc: "next tab",
				callback: func(m *model) tea.Cmd {
					m.activeTab = (m.activeTab + 1) % len(m.tabs)
					m.updateTab()
					return nil
				},
			},
			"shift+tab": {
				desc: "previous tab",
				callback: func(m *model) tea.Cmd {
					m.activeTab = (m.activeTab + len(m.tabs) - 1) % len(tabs)
					m.updateTab()
					return nil
				},
			},
			"/": {
				desc: "search message",
				callback: func(m *model) tea.Cmd {
					m.focused = msgSearch
					m.msgSearch.Focus()
					return nil
				},
			},
			"f": {
				desc: "start date",
				callback: func(m *model) tea.Cmd {
					m.focused = startDate
					m.startDate.Focus()
					return nil
				},
			},
			"e": {
				desc: "end date",
				callback: func(m *model) tea.Cmd {
					m.focused = endDate
					m.endDate.Focus()
					return nil
				},
			},
			"p": {
				desc: "search process",
				callback: func(m *model) tea.Cmd {
					m.focused = processSearch
					m.processSearch.Focus()
					return nil
				},
			},
		},
	}

	m.updateTab()

	p := tea.NewProgram(&m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v", err)
		os.Exit(1)
	}
}

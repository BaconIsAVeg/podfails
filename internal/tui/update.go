package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.header.IsEditing() {
		var cmd tea.Cmd
		m.header, cmd = m.header.Update(msg)
		cmds = append(cmds, cmd)
	}

	var notifCmd tea.Cmd
	m.notification, notifCmd = m.notification.Update(msg)
	cmds = append(cmds, notifCmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.header.SetWidth(m.width)
		m.statusbar.SetWidth(m.width)
		if m.state == stateTable {
			m.table = buildTable(m.issues, m.width, contentHeight(m.height))
		}
		if m.state == stateDetail {
			m.viewport.Width = m.width
			m.viewport.Height = contentHeight(m.height)
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateTable:
			return m.updateTable(msg)
		case stateDetail:
			return m.updateDetail(msg)
		}

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case scanDoneMsg:
		m.issues = msg.issues
		m.err = nil
		m.state = stateTable
		m.table = buildTable(m.issues, m.width, contentHeight(m.height))
		return m, nil

	case scanErrMsg:
		m.err = msg.err
		m.issues = nil
		m.state = stateTable
		m.table = buildTable(nil, m.width, contentHeight(m.height))
		cmd := m.notification.ShowWarning(fmt.Sprintf("Scan failed: %v", msg.err))
		return m, cmd

	case eventsDoneMsg:
		m.events = msg.events
		m.loadingEvents = false
		m.eventErr = nil
		m.viewport.SetContent(m.renderDetail())
		return m, nil

	case eventsErrMsg:
		m.loadingEvents = false
		m.events = nil
		m.eventErr = msg.err
		m.viewport.SetContent(m.renderDetail())
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

func (m Model) updateTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "r":
		m.state = stateLoading
		m.issues = nil
		m.err = nil
		return m, tea.Batch(m.spinner.Tick, scanCmd(m.clients, m.scanOpts()))

	case "enter":
		if len(m.issues) == 0 {
			return m, nil
		}
		row := m.table.SelectedRow()
		if row == nil {
			return m, nil
		}
		idx := m.table.Cursor()
		if idx < 0 || idx >= len(m.issues) {
			return m, nil
		}
		issue := m.issues[idx]
		m.selectedIssue = issue

		vp := viewport.New(m.width, contentHeight(m.height))
		m.viewport = vp
		m.state = stateDetail
		m.loadingEvents = true
		m.events = nil
		m.eventErr = nil
		m.viewport.SetContent(m.renderDetail())

		m.statusbar.SetMode("D")
		m.statusbar.SetKeybindings(detailKeybindings())
		m.statusbar.SetMiddleContent(m.issueBreadcrumb(issue))

		m.header.SetLeft(appTitle)
		m.header.SetMiddle("")

		cc := m.clientForContext(issue.Context)
		if cc == nil {
			m.loadingEvents = false
			return m, nil
		}
		return m, fetchEventsCmd(*cc, issue.Namespace, issue.PodName)

	default:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
}

func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.state = stateTable
		m.header.SetLeft(appTitle)
		m.statusbar.SetMode("L")
		m.statusbar.SetKeybindings(tableKeybindings())
		m.statusbar.SetMiddleContent("")
		return m, nil
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
}

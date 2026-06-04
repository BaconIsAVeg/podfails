package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/BaconIsAVeg/podfails/internal/kube"
)

func (m Model) View() string {
	switch m.state {
	case stateLoading:
		return m.viewLoading()
	case stateTable:
		return m.viewTable()
	case stateDetail:
		return m.viewDetail()
	}
	return ""
}

func (m Model) viewLoading() string {
	msg := fmt.Sprintf("\n\n  %s Scanning %d context(s)...\n\n", m.spinner.View(), len(m.clients))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) viewTable() string {
	m.header.SetLeft(appTitle)
	m.header.SetMiddle(m.filterSummary())
	m.header.SetRight(fmt.Sprintf("%d issue(s)", len(m.issues)))
	m.header.SetWidth(m.width)

	m.statusbar.SetMode("L")
	m.statusbar.SetKeybindings(tableKeybindings())
	m.statusbar.SetMiddleContent(m.version)
	m.statusbar.SetWidth(m.width)

	var body string
	if m.err != nil {
		body = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87")).
			Padding(1, 2).
			Render(fmt.Sprintf("Error: %v", m.err))
	} else if len(m.issues) == 0 {
		body = healthyStyle.Render("✓  All resources are healthy across all scanned contexts.")
	} else {
		body = m.table.View()
	}

	base := lipgloss.JoinVertical(lipgloss.Left,
		m.header.View(),
		body,
		m.statusbar.View(),
	)

	return m.withOverlay(base)
}

func (m Model) viewDetail() string {
	issue := m.selectedIssue
	m.header.SetLeft(appTitle)
	m.header.SetMiddle("")
	m.header.SetRight(issue.Status)
	m.header.SetWidth(m.width)

	m.statusbar.SetMode("D")
	m.statusbar.SetKeybindings(detailKeybindings())
	m.statusbar.SetMiddleContent(m.issueBreadcrumb(issue))
	m.statusbar.SetWidth(m.width)

	base := lipgloss.JoinVertical(lipgloss.Left,
		m.header.View(),
		m.viewport.View(),
		m.statusbar.View(),
	)

	return m.withOverlay(base)
}

func (m Model) renderDetail() string {
	issue := m.selectedIssue
	statusRendered := statusStyle(issue.Status).Render(issue.Status)

	sb := &strings.Builder{}
	fmt.Fprint(sb, "\n")
	fmt.Fprintf(sb, "  Name:      %s\n", issue.Name)
	fmt.Fprintf(sb, "  Namespace: %s\n", issue.Namespace)
	fmt.Fprintf(sb, "  Context:   %s\n", issue.Context)
	fmt.Fprintf(sb, "  Kind:      %s\n", issue.Kind)
	fmt.Fprintf(sb, "  Status:    %s\n", statusRendered)
	if issue.Reason != "" {
		fmt.Fprintf(sb, "  Reason:    %s\n", issue.Reason)
	}
	if issue.Kind == kube.KindHPA {
		fmt.Fprintf(sb, "  Minimum:   %d\n", issue.MinReplicas)
		fmt.Fprintf(sb, "  Maximum:   %d\n", issue.MaxReplicas)
		fmt.Fprintf(sb, "  Replicas:  %d\n", issue.CurrentReplicas)
	} else if issue.Metric != "" {
		fmt.Fprintf(sb, "  Restarts:  %s\n", issue.Metric)
	}
	fmt.Fprintf(sb, "  Age:       %s\n", kube.FormatAge(issue.Age))

	if len(issue.Conditions) > 0 {
		sb.WriteString("\n")
		dividerWidth := max(m.width-4, 20)
		sb.WriteString(dividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
		sb.WriteString("\n  Conditions\n")
		sb.WriteString(dividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
		sb.WriteString("\n")

		typeW := 18
		statusW := 6
		reasonW := 28
		ageW := 8
		msgW := max(dividerWidth-typeW-statusW-reasonW-ageW-14, 10)
		fmt.Fprintf(sb, "  %-*s  %-*s  %-*s  %-*s  %s\n",
			typeW, "TYPE", statusW, "STATUS", reasonW, "REASON", msgW, "MESSAGE", "AGE")
		sb.WriteString(dividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
		sb.WriteString("\n")
		for _, cond := range issue.Conditions {
			msg := truncate(cond.Message, msgW)
			line := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s  %s",
				typeW, truncate(cond.Type, typeW),
				statusW, cond.Status,
				reasonW, truncate(cond.Reason, reasonW),
				msgW, msg,
				kube.FormatAge(cond.Age))
			var style lipgloss.Style
			if cond.Status == "True" {
				style = warningEventStyle
			} else {
				style = normalEventStyle
			}
			sb.WriteString(style.Render(line))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	dividerWidth := max(m.width-4, 20)
	sb.WriteString(dividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	sb.WriteString("\n  Events\n")
	sb.WriteString(dividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
	sb.WriteString("\n")

	if m.loadingEvents {
		sb.WriteString("  Loading events...\n")
	} else if m.eventErr != nil {
		fmt.Fprintf(sb, "  Error loading events: %v\n", m.eventErr)
	} else if len(m.events) == 0 {
		sb.WriteString("  No events found.\n")
	} else {
		typeW := 8
		reasonW := 22
		ageW := 8
		msgW := max(dividerWidth-typeW-reasonW-ageW-10, 10)
		fmt.Fprintf(sb, "  %-*s  %-*s  %-*s  %s\n", typeW, "TYPE", reasonW, "REASON", msgW, "MESSAGE", "AGE")
		sb.WriteString(dividerStyle.Render("  " + strings.Repeat("─", dividerWidth)))
		sb.WriteString("\n")
		for _, ev := range m.events {
			msg := truncate(ev.Message, msgW)
			line := fmt.Sprintf("  %-*s  %-*s  %-*s  %s",
				typeW, ev.Type, reasonW, truncate(ev.Reason, reasonW), msgW, msg, kube.FormatAge(ev.Age))
			var style lipgloss.Style
			if ev.Type == "Warning" {
				style = warningEventStyle
			} else {
				style = normalEventStyle
			}
			sb.WriteString(style.Render(line))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/BaconIsAVeg/github-tuis/ui/helpers"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/BaconIsAVeg/podfails/internal/kube"
)

const contextColWidth = 24

func contentHeight(termHeight int) int {
	h := termHeight - headerHeight - statusbarHeight
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) scanOpts() kube.ScanOptions {
	return kube.ScanOptions{PodRegex: m.podRegex, Namespace: m.namespace}
}

func (m Model) issueBreadcrumb(iss kube.Issue) string {
	return fmt.Sprintf("%s / %s / %s", truncateStart(iss.Context, contextColWidth), iss.Namespace, iss.Name)
}

func (m Model) withOverlay(base string) string {
	if m.notification.Visible() {
		notifView := m.notification.View()
		notifWidth := lipgloss.Width(notifView)
		return helpers.PlaceOverlay(m.width-notifWidth-1, m.height-2, notifView, base, true, palette.ShadowFg)
	}
	return base
}

func buildTable(issues []kube.Issue, width, height int) table.Model {
	contextW := contextColWidth
	nsW := 18
	statusW := 24
	metricW := 9
	ageW := 7
	nameW := width - contextW - nsW - statusW - metricW - ageW - 10
	if nameW < 20 {
		nameW = 20
	}

	cols := []table.Column{
		{Title: "CONTEXT", Width: contextW},
		{Title: "NAMESPACE", Width: nsW},
		{Title: "NAME", Width: nameW},
		{Title: "STATUS", Width: statusW},
		{Title: "METRIC", Width: metricW},
		{Title: "AGE", Width: ageW},
	}

	rows := make([]table.Row, len(issues))
	for i, iss := range issues {
		rows[i] = table.Row{
			truncateStart(iss.Context, contextW),
			truncate(iss.Namespace, nsW),
			truncate(iss.Name, nameW),
			truncate(iss.Status, statusW),
			iss.Metric,
			kube.FormatAge(iss.Age),
		}
	}

	s := table.DefaultStyles()
	s.Header = palette.Header
	s.Selected = selectedRowStyle

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(height),
		table.WithStyles(s),
	)
	return t
}

func (m Model) clientForContext(name string) *kube.ContextClient {
	for i := range m.clients {
		if m.clients[i].Name == name {
			return &m.clients[i]
		}
	}
	return nil
}

func (m Model) filterSummary() string {
	var parts []string
	if m.contextRegex != "" {
		parts = append(parts, filterContextStyle.Render("ctx:"+m.contextRegex))
	}
	if m.podRegex != "" {
		parts = append(parts, filterPodStyle.Render("pod:"+m.podRegex))
	}
	if m.namespace != "" {
		parts = append(parts, filterNamespaceStyle.Render("ns:"+m.namespace))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func truncateStart(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if maxRunes <= 3 {
		return string(runes[len(runes)-maxRunes:])
	}
	return "..." + string(runes[len(runes)-(maxRunes-3):])
}

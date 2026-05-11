package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/BaconIsAVeg/podfails/internal/kube"
)

type scanDoneMsg struct{ issues []kube.PodIssue }
type scanErrMsg struct{ err error }
type eventsDoneMsg struct{ events []kube.Event }
type eventsErrMsg struct{ err error }

func scanCmd(clients []kube.ContextClient, opts kube.ScanOptions) tea.Cmd {
	return func() tea.Msg {
		issues, err := kube.ScanAll(clients, opts)
		if err != nil {
			return scanErrMsg{err: err}
		}
		return scanDoneMsg{issues: issues}
	}
}

func fetchEventsCmd(cc kube.ContextClient, namespace, podName string) tea.Cmd {
	return func() tea.Msg {
		events, err := kube.GetPodEvents(cc.Client, namespace, podName)
		if err != nil {
			return eventsErrMsg{err: err}
		}
		return eventsDoneMsg{events: events}
	}
}

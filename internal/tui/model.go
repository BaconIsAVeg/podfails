package tui

import (
	"github.com/BaconIsAVeg/github-tuis/buildinfo"
	"github.com/BaconIsAVeg/github-tuis/ui/header"
	"github.com/BaconIsAVeg/github-tuis/ui/notification"
	"github.com/BaconIsAVeg/github-tuis/ui/statusbar"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BaconIsAVeg/podfails/internal/kube"
)

const (
	appTitle        = "☸ podfails"
	headerHeight    = 1
	statusbarHeight = 1
)

type state int

const (
	stateLoading state = iota
	stateTable
	stateDetail
)

type Model struct {
	state         state
	clients       []kube.ContextClient
	issues        []kube.PodIssue
	events        []kube.Event
	table         table.Model
	viewport      viewport.Model
	spinner       spinner.Model
	header        header.Model
	statusbar     statusbar.Model
	notification  notification.Model
	width         int
	height        int
	err           error
	eventErr      error
	contextRegex  string
	podRegex      string
	namespace     string
	selectedIssue kube.PodIssue
	loadingEvents bool
	version       string
}

func NewModel(clients []kube.ContextClient, contextRegex string, opts kube.ScanOptions) Model {
	initPalette()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5FD6"))

	h := header.New(palette)
	h.SetLeft(appTitle)

	sb := statusbar.New(palette)
	sb.SetMode("L")

	notif := notification.New(palette)

	return Model{
		state:        stateLoading,
		clients:      clients,
		spinner:      s,
		header:       h,
		statusbar:    sb,
		notification: notif,
		contextRegex: contextRegex,
		podRegex:     opts.PodRegex,
		namespace:    opts.Namespace,
		version:      buildinfo.GetVersion(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scanCmd(m.clients, m.scanOpts()))
}

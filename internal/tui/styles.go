package tui

import (
	"sync"

	"github.com/BaconIsAVeg/github-tuis/ui/styles"
	"github.com/charmbracelet/lipgloss"

	"github.com/BaconIsAVeg/podfails/internal/kube"
)

var palette *styles.Palette
var paletteInit sync.Once

func initPalette() {
	paletteInit.Do(func() {
		palette = styles.NewPalette(lipgloss.HasDarkBackground())
	})
}

var (
	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444"))

	healthyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			Padding(1, 2)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#5C5FD6"))

	filterContextStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("22")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	filterPodStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("25")).
			Foreground(lipgloss.Color("15")).
			Bold(true).
			Padding(0, 1)

	filterNamespaceStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("58")).
				Foreground(lipgloss.Color("15")).
				Bold(true).
				Padding(0, 1)

	warningEventStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F87"))
	normalEventStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))

	statusCriticalStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF5F87"))
	statusWarningStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFCC00"))
	statusInfoStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7D7"))
	statusDefaultStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA"))
)

var statusStyleMap = map[string]lipgloss.Style{
	kube.StatusCrashLoopBackOff:           statusCriticalStyle,
	kube.StatusOOMKilled:                  statusCriticalStyle,
	kube.StatusError:                      statusCriticalStyle,
	kube.StatusFailed:                     statusCriticalStyle,
	kube.StatusScanError:                  statusCriticalStyle,
	kube.StatusImagePullBackOff:           statusWarningStyle,
	kube.StatusErrImagePull:               statusWarningStyle,
	kube.StatusCreateContainerConfigError: statusWarningStyle,
	kube.StatusRunContainerError:          statusWarningStyle,
	kube.StatusPending:                    statusWarningStyle,
	kube.StatusHighRestarts:               statusWarningStyle,
	kube.StatusUnknown:                    statusInfoStyle,
}

func statusStyle(status string) lipgloss.Style {
	if s, ok := statusStyleMap[status]; ok {
		return s
	}
	return statusDefaultStyle
}

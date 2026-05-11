package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/BaconIsAVeg/podfails/internal/kube"
	"github.com/BaconIsAVeg/podfails/internal/tui"
)

var (
	contextRegex string
	podRegex     string
	namespace    string
)

var rootCmd = &cobra.Command{
	Use:   "podfails",
	Short: "Monitor troubled Kubernetes pods across multiple contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		clients, err := kube.LoadContexts(contextRegex)
		if err != nil {
			return fmt.Errorf("loading contexts: %w", err)
		}
		if len(clients) == 0 {
			fmt.Fprintln(os.Stderr, "No kubeconfig contexts found (check --context or KUBECONFIG).")
			return nil
		}

		opts := kube.ScanOptions{
			PodRegex:  podRegex,
			Namespace: namespace,
		}
		m := tui.NewModel(clients, contextRegex, opts)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.Flags().StringVarP(&contextRegex, "context", "c", "", "Regex to filter context names (e.g. 'prod$', 'apps')")
	rootCmd.Flags().StringVarP(&podRegex, "pods", "p", "", "Regex to filter pod names (e.g. 'api-', 'web.*')")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to limit scanning (default: all namespaces)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

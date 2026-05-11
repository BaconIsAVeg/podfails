# podfails

Kubernetes pod health monitor with a terminal TUI (Bubble Tea).

## Build & verify

```sh
make build          # go build -o podfails ./cmd/app
make fmt            # go fmt ./...
make vet            # go vet ./...
```

Run `make fmt && make vet` after code changes. No tests exist yet.

## Architecture

- **Entry point**: `cmd/app/main.go` — Cobra CLI with flags `-c` (context regex), `-p` (pod regex), `-n` (namespace)
- `internal/kube` — K8s client loading, pod scanning, event fetching
- `internal/tui` — Bubble Tea TUI (model/view/update), styles, keybindings

## Key conventions

- Uses `github.com/BaconIsAVeg/github-tuis/ui` for shared UI primitives (`header`, `statusbar`, `notification`, `styles.Palette`). Don't replace these with raw Bubble Tea widgets.
- Import path is `podfails/internal/...` (matches `go.mod` module name).
- Running the binary requires a valid kubeconfig with reachable contexts; there is no mock/offline mode.

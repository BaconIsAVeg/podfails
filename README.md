# podfails

A terminal TUI that scans your Kubernetes contexts and surfaces troubled pods — crashes, image pull failures, OOM kills, and more.

## Installation

### Go install

```sh
go install github.com/BaconIsAVeg/podfails/cmd/podfails@latest
```

This installs the `podfails` binary to `$GOPATH/bin` (or `$HOME/go/bin`).

### From source

```sh
go build -o podfails ./cmd/app
```

Or install to a directory on your PATH:

```sh
make install   # installs to ~/.local/bin
```

## Usage

```sh
podfails [flags]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--context` | `-c` | Regex to filter kubeconfig context names (e.g. `prod$`, `panel`) |
| `--pods` | `-p` | Regex to filter pod names (e.g. `api-`, `web.*`) |
| `--namespace` | `-n` | Namespace to limit scanning (default: all namespaces) |

### Examples

Scan all contexts:
```sh
podfails
```

Scan only contexts matching "prod":
```sh
podfails -c prod
```

Scan a specific namespace for pods matching a pattern:
```sh
podfails -n monitoring -p prometheus
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate rows |
| `enter` | View pod details and events |
| `esc` | Back to pod list |
| `r` | Refresh scan |
| `q` | Quit |

## Prerequisites

A valid kubeconfig with reachable Kubernetes contexts. Respects the `KUBECONFIG` environment variable.

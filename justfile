binary      := "podfails"
install_dir := env_var("HOME") / ".local" / "bin"
default     : build

# compile the binary into the project root
build:
    go build -o {{ binary }} ./cmd/podfails

# build and install the binary to install_dir
install: build
    mkdir -p {{ install_dir }}
    cp {{ binary }} {{ install_dir / binary }}
    echo "Installed to {{ install_dir / binary }}"

# remove the installed binary
uninstall:
    rm -f {{ install_dir / binary }}
    echo "Removed {{ install_dir / binary }}"

# remove the compiled binary
clean:
    rm -f {{ binary }}

# tidy Go module dependencies
tidy:
    go mod tidy

# format all Go source files
fmt:
    go fmt ./...

# run go vet
vet:
    go vet ./...

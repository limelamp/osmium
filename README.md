# Osmium

Osmium is a terminal-based Minecraft server manager built in Go with Bubble Tea, Lip Gloss, Bubbles, and Cobra. It is designed to help you organize, inspect, and manage multiple server instances from a single multipage TUI.

## Highlights

- Multipage terminal UI for navigating server-related workflows.
- Persistent local storage for managed servers.
- Designed for Minecraft server administration and organization.
- Built with a modern Go TUI stack: Bubble Tea v2, Lip Gloss, Bubbles, and Cobra.

## Requirements

- Go 1.25.3 or newer.
- A supported terminal with UTF-8 and color support.
- A local filesystem location where Osmium can store its config and data.

## Installation

Clone the repository and build the TUI from the `tui` module:

```bash
git clone https://github.com/limelamp/osmium-refactor.git
cd osmium
go build ./...
```

If you want a runnable binary, build the module directly:

```bash
go build -o osmium
```

## Running

Start the application from the `tui` module directory:

```bash
go run .
```

The project also exposes a Cobra command surface, so once built you can run the binary directly:

```bash
./osmium
```

## Data Storage

Osmium stores its application data in the user config directory under an `osmium` folder.

On Linux, that is typically:

```text
~/.config/osmium
```

Managed server metadata is saved in:

```text
~/.config/osmium/servers.json
```

If the platform config directory cannot be resolved, Osmium falls back to a hidden folder in the user home directory.

## Project Structure

```text
tui/
├── cmd/                 # Cobra commands and app entrypoints
├── internal/assets/     # Embedded assets such as the logo
├── internal/tui/        # Bubble Tea app, pages, components, styles, and storage
└── main.go              # Program entrypoint
```

## Development

Useful commands while iterating on the TUI:

```bash
go fmt ./...
go test ./...
go build ./...
```

## Contributing

Contributions are welcome. Keep changes focused, follow the existing project structure, and update documentation when behavior changes.

## License

Add the project license here once it is finalized.
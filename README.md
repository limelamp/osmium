<div align="center">

```
 ██████  ███████ ███    ███ ██ ██    ██ ███    ███
██    ██ ██      ████  ████ ██ ██    ██ ████  ████
██    ██ ███████ ██ ████ ██ ██ ██    ██ ██ ████ ██
██    ██      ██ ██  ██  ██ ██ ██    ██ ██  ██  ██
 ██████  ███████ ██      ██ ██  ██████  ██      ██
```

**A terminal Minecraft server manager.**

Create servers, run them, install mods, watch their consoles - from one terminal, or from a script.

[![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Licence](https://img.shields.io/badge/licence-GPL--3.0-blue)](LICENSE)
[![Release](https://img.shields.io/github/v/release/limelamp/osmium?display_name=tag&sort=semver)](https://github.com/limelamp/osmium/releases)
[![Go report](https://goreportcard.com/badge/github.com/limelamp/osmium)](https://goreportcard.com/report/github.com/limelamp/osmium)

</div>

---

## What it is

Osmium aids in the creation and the managing of Minecraft servers, all done from the terminal. It downloads the server software, keeps track of where each server lives and what/how it runs, installs mods and plugins from Modrinth automatically, and gives every server a console you can see and type into.

Everything the interface does, a command does too. `osmium` opens the full-screen interface; 
`osmium start -s survival` starts a server from a script.

## Screenshots

The home page lists what is running:

<img width="1313" height="837" alt="Screenshot_20260830_031757" src="https://github.com/user-attachments/assets/3fa8f6e4-7ade-4a72-9f97-3dd8cf3bc335" />



Manage Servers puts the list, the actions and the server's console on one screen:

<img width="1313" height="837" alt="Screenshot_20260830_031843" src="https://github.com/user-attachments/assets/25a4c638-6624-4ee0-abf1-3a65f4abbebc" />



The create wizard walks through the choices, one at a time:

<img width="1313" height="837" alt="Screenshot_20260830_031951" src="https://github.com/user-attachments/assets/5400497c-0deb-4d5d-a618-dab38565e361" />


## Requirements

- **Java** - whatever your server software needs (Java 21 for Minecraft 1.20.5 and newer, lookup the rest yourself).
- A terminal with UTF-8 and 256 colours. Minimum size is **60 × 18**.
- If building, use **Go 1.25+**.

## Installing   

```bash
go install github.com/limelamp/osmium@latest
```

Or build from source:

```bash
git clone https://github.com/limelamp/osmium.git
cd osmium
make build           # ./bin/osmium
make install         # into $(go env GOPATH)/bin
```

Or take a binary from the [releases](https://github.com/limelamp/osmium/releases) - Linux, macOS and
Windows, amd64 and arm64.

Then to set up tab completion, run the command below once:

```bash
osmium completion install
```

It completes server names, your installed mods and plugins, engines and Minecraft versions.

## Quick start

```bash
osmium                                   # the interface
osmium create                            # a wizard, or give it flags:
osmium create --name survival --engine Paper --version 1.21.1 --ram 4G --eula

osmium start   -s survival               # start it; it keeps running when you close Osmium
osmium logs    -s survival -f            # watch it
osmium exec    -s survival say hello     # talk to it
osmium mod add sodium -s survival        # install a mod from Modrinth
osmium stop    -s survival               # ask it to shut down and save
```

`--server`/`-s` names a registered server. Leave it out and the command acts on the folder you are
standing in, so you can `cd` into a server and type `osmium logs -f`.

Running `osmium -s survival` opens the interface straight onto that server. Running it inside a
server folder does the same.

## Commands

**The server itself**

| Command | What it does |
|---|---|
| `osmium` | Open the interface (tui). `-s <name>` opens it on one server |
| `osmium create` | Create a server - a wizard, or fully specified by flags |
| `osmium list` | Every registered server, what it runs, and whether it is up |
| `osmium details` | One server in full: engine, disk, players, ops, bans, mods |
| `osmium start` / `stop` / `restart` | Run it, ask it to shut down, or both |
| `osmium exec <command…>` | Send a console command to a running server |
| `osmium logs [-f] [-n 200]` | Read or follow a server's output |
| `osmium edit --name … --ram …` | Change what Osmium keeps: the name and the memory |
| `osmium migrate [-l Paper] [-v 1.21.1]` | Move a server to another loader or version |
| `osmium destroy` | Delete a server, after typing its name |

**Mods and plugins**

| Command | What it does |
|---|---|
| `osmium mod add <slug…>` | Install from Modrinth and track it |
| `osmium mod remove <slug…>` | Delete it and stop tracking it |
| `osmium mod list` | What is tracked, and which build |
| `osmium mod install` | Install everything `osmium.json` lists |
| `osmium mod update [slug…]` | Update to the newest compatible builds |
| `osmium mod track` | Adopt jars already sitting in `mods/` |

`osmium plugin …` is the same, for `plugins/`.

Every command takes `-s/--server`. `osmium <command> --help` has the details.

## Controls

| Key | Where | What |
|---|---|---|
| `↑ ↓` `j k` | anywhere | Move; lists wrap around at the ends |
| `enter` | anywhere | Choose, or open a field for typing |
| `esc` | anywhere | Cancel what you are typing, else go back |
| `tab` | Manage Servers, Settings | Move between panes |
| `ctrl+b` | Manage Servers | Hide and show the servers pane |
| `ctrl+u` | any field | Clear it |
| `end` | Activity | Jump to the newest line and resume following |
| `space` `ctrl+a` | file and mod lists | Mark one, mark all |
| `?` | anywhere | The keys that work right here |
| `q` `ctrl+c` | home / anywhere | Quit |

The mouse works too: click to focus a pane, click a row to select it, click again to open it, and the wheel scrolls whatever is under the pointer.

## Development

```bash
make            # list the targets
make run        # go run .
make test       # go test ./...
make lint       # golangci-lint, configured in .golangci.yml
make fmt        # gofumpt + goimports
make snapshot   # build the release artefacts locally
```

The layout:

```
cmd/                the command line, one file per command
internal/build/     what the binary knows about itself
internal/config/    config.json and osmium.json
internal/scope/     where an operation runs, and where its output goes
internal/shared/    Modrinth, migration, player records
internal/supervisor/the detached process that owns a running server
internal/util/      downloading and installing server software
internal/tui/       the interface
    core/           layout, lists, scrolling, messages, fields, tickers
    styles/         containers, rows, hints, logs, progress bars
    components/     the panels
    pages/          the screens
```

Pull requests are welcome. `make lint test` should pass before you open one.

## License

Osmium is free software under the [GNU General Public License v3.0](LICENSE).

Copyright © 2026 **Limelamp**.

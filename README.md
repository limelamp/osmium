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

Osmium manages Minecraft servers from the terminal. It downloads the server software, keeps track of
where each server lives and what it runs, installs mods and plugins from Modrinth, and gives every
server a console you can type into.

Each server runs under a supervisor process of its own, so **quitting Osmium does not stop your
servers** - reopen it tomorrow and the console is still there, still following, still able to send
commands.

Everything the interface does, a command does too. `osmium` opens the full-screen interface;
`osmium start -s survival` starts a server from a script.

## Screens

The home page lists what is running:

```
                      ██████  ███████ ███    ███ ██ ██    ██ ███    ███
                     ██    ██ ██      ████  ████ ██ ██    ██ ████  ████
                     ██    ██ ███████ ██ ████ ██ ██ ██    ██ ██ ████ ██
                     ██    ██      ██ ██  ██  ██ ██ ██    ██ ██  ██  ██
                      ██████  ███████ ██      ██ ██  ██████  ██      ██

                     ┌ Menu ──────────────────────────────────────────┐
                     │ ➔ Create server                              c │
                     │   Manage servers                             m │
                     │   Migrate server                             g │
                     │   Settings                                   s │
                     │   Quit                                       q │
                     └────────────────────────────────────────────────┘
                     ┌ Running ───────────────────────────────────────┐
                     │   ● survival                             up 9s │
                     └────────────────────────────────────────────────┘
                     [↑/↓] move  •  [enter] open  •  [?] help
```

Manage Servers puts the list, the actions and the server's console on one screen:

```
│Osmium                                                                      Manage Servers│
┌ Servers ──────────────────┐┌ Actions · survival ─────────────────────────────────────────┐
│ ➔ ● survival       up 50s ││ SERVER                                                      ┃
│   ○ creative              ││ ➔ Server details                               Paper 1.21.1 ┃
│   ○ testbed               ││   Players                                           0 known ┃
│                           ││                                                             ┃
│                           ││ PROCESS                                                     ┃
│                           ││   Start server                                    ● running ┃
│                           ││   Stop server                                               ┃
│                           ││   Restart server                                            ┃
│                           ││                                                             │
│                           ││ CONTENT                                                     │
│                           ││   Mods                                                      │
│                           ││   Plugins                                                   │
│                           ││   Install everything tracked                                │
│                           ││   Track installed files                                     │
│                           ││                                                             │
│                           ││ [↑/↓] move  •  [enter] run  •  [ctrl+b] hide servers        │
│                           │└─────────────────────────────────────────────────────────────┘
│                           │┌ Activity · survival ────────────────────────────────────────┐
│                           ││ you    │ › list                                             │
│                           ││ server │ [23:03:44 INFO]: There are 0 of a max of 20        │
│                           ││          players online:                                    │
│                           ││ ─────────────────────────────────────────────────────────── │
│ [↑/↓] select              ││ › Enter a command...                                        ┃
└───────────────────────────┘└─────────────────────────────────────────────────────────────┘
```

Server details gathers everything about one server, including who is on it:

```
┌ Server Details · survival ──────────────────────────────────┐
│ STATUS                                                      ┃
│   state         running   up 1m · pid 3191536               ┃
│   memory        2G                                          ┃
│                                                             ┃
│ SERVER                                                      ┃
│   engine        Paper 1.21.1                                ┃
│   folder        ~/servers/survival                          ┃
│   size on disk  217.7 MB                                    ┃
│                                                             ┃
│ PLAYERS                                                     ┃
│   online        0 / 20                                      ┃
│   ops           -                                           ┃
│   banned        -                                           ┃
│                                                             ┃
│ CONTENT                                                     │
│ [↑/↓] scroll  •  [r] refresh  •  [backspace] back           │
└─────────────────────────────────────────────────────────────┘
```

The create wizard walks through the choices, one at a time:

```
┌ Setup New Server ────────────────────────────────────────────────────────────────────────┐
│ 3/8 Engine  ━━━─────                                                                     │
│                                                                                          │
│ Step 3 - Choose Server Engine                                                            │
│ Pick the software your server runs on.                                                   │
│                                                                                          │
│ Vanilla          Plugin           Mod Loader                                             │
│                                                                                          │
│ ➔ Vanilla          Paper            Forge                                                │
│                    Purpur           NeoForge                                             │
│                    Spigot           Fabric                                               │
│                    Bukkit           Quilt                                                │
│                                                                                          │
│ The official, unmodified Minecraft server software.                                      │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

## Requirements

- **Java** - whatever your server software needs (Java 21 for Minecraft 1.20.5 and newer).
- A terminal with UTF-8 and 256 colours. Minimum size **60 × 18**; Osmium says so if it has less.
- **Go 1.25+**, only if you are building it yourself.

## Install

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

Then set up tab completion, once:

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
| `osmium` | Open the interface. `-s <name>` opens it on one server |
| `osmium create` | Create a server - a wizard, or fully specified by flags |
| `osmium list` | Every registered server, what it runs, and whether it is up |
| `osmium details` | One server in full: engine, disk, players, ops, bans, mods |
| `osmium start` / `stop` / `restart` | Run it, ask it to shut down, or both |
| `osmium exec <command…>` | Send a console command to a running server |
| `osmium logs [-f] [-n 200]` | Read or follow a server's output |
| `osmium edit --name … --ram …` | Change what Osmium keeps: the name and the memory |
| `osmium migrate [-l Paper] [-v 1.21.1]` | Move a server to another loader or version |
| `osmium destroy` | Delete a server, after typing its name |

**Mods and plugins** - the same six verbs for each:

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

## How servers keep running

Starting a server spawns a **supervisor**: a detached `osmium serve` process that owns the server,
and nothing else. It holds the server's stdin, writes everything the server says to
`<server>/.osmium/server.log`, and listens on a control socket in the server's own folder.

That is what makes the rest work:

- Quitting Osmium leaves your servers running.
- Reopening it reattaches - the console shows the last thousand lines and carries on following.
- `osmium exec` reaches a server whoever started it, from any terminal.
- Two servers never collide: one supervisor, one socket, one log, per folder.

Stopping a server sends the console's own `stop`, so it saves its world; `--force` kills it.

## Files

| Path | What it is |
|---|---|
| `~/.config/osmium/config.json` | Theme, default server folder, default memory |
| `~/.config/osmium/servers.json` | The registry: name, path, engine, version, memory |
| `<server>/osmium.json` | The server's own manifest, and the truth about it |
| `<server>/.osmium/server.log` | Everything that server has said, rotated at 4 MB |
| `<server>/.osmium/control.sock` | Where its supervisor listens (a loopback port on Windows) |
| `<server>/.osmium_process.lock` | The supervisor's PID, and how "running" is decided |

A server is its folder. Delete the folder and Osmium drops it from the list next time it looks;
point Osmium at a folder holding an `osmium.json` and it adopts it.

## Keys

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

The mouse works too: click to focus a pane, click a row to select it, click again to open it, and
the wheel scrolls whatever is under the pointer.

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

## Licence

Osmium is free software under the [GNU General Public License v3.0](LICENSE).

Copyright © 2026 **Limelamp**.

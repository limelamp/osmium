I build a multipage CLI/TUI app written in Go 1.26 using bubbletea v2, lipgloss, bubbles, and cobra. It's a minecraft server management app which stores its config, data and etc in appdata directory.

Folder structure:
osmium
├── cmd
│   ├── root.go
│   └── tui.go
├── colors.md
├── feedback.md
├── go.mod
├── go.sum
├── internal
│   ├── assets
│   │   └── logo.go
│   └── tui
│       ├── app.go
│       ├── components
│       │   ├── actions.go
│       │   ├── activity.go
│       │   ├── filters.go
│       │   ├── navigation.go
│       │   ├── server_details.go
│       │   └── servers.go
│       ├── config
│       ├── constants
│       │   └── server.go
│       ├── core
│       │   ├── layout.go
│       │   └── messages.go
│       ├── pages
│       │   ├── create_server2.go
│       │   ├── create_server.go
│       │   ├── dashboard.go
│       │   ├── home.go
│       │   ├── manage_servers.go
│       │   ├── server_files.go
│       │   └── settings.go
│       ├── storage
│       │   └── servers.go
│       ├── styles
│       │   ├── app.go
│       │   └── colors.go
│       └── theme
├── main.go

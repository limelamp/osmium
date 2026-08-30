package actions

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/backup"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/limelamp/osmium/internal/scope"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/storage"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// BackupsModel is a server's snapshots: what has been kept, and what to do
// with them.
//
// A world is the one part of a server that cannot be downloaded again, which
// is the whole argument for this panel existing beside the ones that download
// things.
type BackupsModel struct {
	base

	backups []backup.Backup
	filter  core.Filter

	// confirming is the backup a restore is waiting to be agreed to.
	confirming *backup.Backup
}

// NewBackupsModel returns the backups panel.
func NewBackupsModel() BackupsModel {
	return BackupsModel{base: newBase(), filter: core.NewFilter()}
}

// Editing reports whether the filter has the keyboard.
func (m BackupsModel) Editing() bool { return m.filter.IsOpen() }

func (m BackupsModel) Init() tea.Cmd { return nil }

func (m BackupsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.common(msg); handled {
		m.base = updated
		return m, cmd
	}

	switch msg := msg.(type) {
	case core.OperationDoneMsg:
		updated, cmd := m.finished(msg)
		m.base = updated
		return m.reload(), cmd

	case core.RefreshMsg:
		return m.reload(), nil

	case tea.MouseMsg:
		updated, activated := m.handleMouse(msg)
		m.base = updated

		if activated {
			return m.activate()
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.confirming != nil {
			switch msg.String() {
			case "enter":
				chosen := *m.confirming
				m.confirming = nil
				return m.restore(chosen)
			default:
				m.confirming = nil
				return m, nil
			}
		}

		if m.filter.IsOpen() {
			filter, cmd := m.filter.HandleKey(msg)
			m.filter = filter
			m.base = m.setCount(len(m.visible()))

			return m, cmd
		}

		if list, handled := m.list.HandleKey(msg.String()); handled {
			m.list = list
			return m, nil
		}

		switch msg.String() {
		case "/":
			filter, cmd := m.filter.Open()
			m.filter = filter
			return m, cmd

		case "n":
			return m.take()
		case "d":
			return m.forget()
		case "enter":
			return m.activate()
		case "backspace":
			return m, core.CloseAction()
		}
	}

	return m, nil
}

// activate asks about restoring whatever the cursor is on.
func (m BackupsModel) activate() (tea.Model, tea.Cmd) {
	chosen, ok := m.chosen()
	if !ok {
		return m, nil
	}

	if supervisor.Running(m.dir()) {
		updated, cmd := m.report("stop the server before restoring a backup", true)
		m.base = updated
		return m, cmd
	}

	m.confirming = &chosen
	return m, nil
}

// take makes a new backup.
func (m BackupsModel) take() (tea.Model, tea.Cmd) {
	updated, cmd := m.begin(0, "copying the world", func(sc scope.Scope) error {
		taken, err := backup.Take(sc.Dir, "")
		if err != nil {
			return err
		}

		sc.Logf("backed up to %s (%s)", taken.Name, pack.FormatBytes(taken.Size))
		return nil
	})

	m.base = updated
	return m, cmd
}

// restore puts a backup back.
func (m BackupsModel) restore(chosen backup.Backup) (tea.Model, tea.Cmd) {
	updated, cmd := m.begin(m.list.Cursor, "restoring "+chosen.Name, func(sc scope.Scope) error {
		if err := backup.Restore(sc.Dir, chosen); err != nil {
			return err
		}

		sc.Logf("restored %s", chosen.Name)
		return nil
	})

	m.base = updated
	return m, cmd
}

// forget deletes a backup.
func (m BackupsModel) forget() (tea.Model, tea.Cmd) {
	chosen, ok := m.chosen()
	if !ok {
		return m, nil
	}

	if err := backup.Remove(chosen); err != nil {
		updated, cmd := m.report(err.Error(), true)
		m.base = updated
		return m, cmd
	}

	updated, cmd := m.report("removed "+chosen.Name, false)
	m.base = updated

	return m.reload(), cmd
}

// reload rereads what has been kept.
func (m BackupsModel) reload() BackupsModel {
	m.backups = backup.List(m.dir())
	m.base = m.setCount(len(m.visible()))

	return m
}

// visible is the list as the filter leaves it.
func (m BackupsModel) visible() []backup.Backup {
	if !m.filter.Active() {
		return m.backups
	}

	kept := make([]backup.Backup, 0, len(m.backups))
	for _, taken := range m.backups {
		if m.filter.Matches(taken.Name, taken.Age()) {
			kept = append(kept, taken)
		}
	}
	return kept
}

// chosen is the backup under the cursor.
func (m BackupsModel) chosen() (backup.Backup, bool) {
	visible := m.visible()
	if len(visible) == 0 || m.list.Cursor >= len(visible) {
		return backup.Backup{}, false
	}
	return visible[m.list.Cursor], true
}

func (m BackupsModel) View() tea.View {
	if m.needsServer() {
		return m.noServerView(m.Title())
	}

	muted := lipgloss.NewStyle().Foreground(theme.Inactive)

	if m.confirming != nil {
		return m.panelView(m.Title()+" · Restore", lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(theme.Primary).Render("Restore "+m.confirming.Name+"?"),
			"",
			muted.Render("Taken "+m.confirming.Age()+"."),
			muted.Render("This writes the world as it was then over the world as it is now."),
			"",
			muted.Render("[enter] restore   [esc] cancel"),
		), m.Hints()...)
	}

	if len(m.backups) == 0 {
		return m.emptyView(m.Title(),
			"No backups yet. Press n to take one — the world, the settings and who is allowed on.",
			styles.Hint{Key: "n", Desc: "new"}, styles.Hint{Key: "backspace", Desc: "back"})
	}

	body := m.list.Render(func(i int) string {
		visible := m.visible()
		if i >= len(visible) {
			return ""
		}

		taken := visible[i]
		return m.row(i, m.rowMarker(i)+taken.Age(),
			fmt.Sprintf("%s   %s", pack.FormatBytes(taken.Size), taken.Name))
	})

	if line := m.filter.View(); line != "" {
		body = line + "\n" + body
	}

	return m.panelView(m.Title(), body, m.Hints()...)
}

// Hints are the keys this panel responds to.
func (m BackupsModel) Hints() []styles.Hint {
	if m.confirming != nil {
		return []styles.Hint{
			{Key: "enter", Desc: "restore"},
			{Key: "esc", Desc: "cancel"},
		}
	}

	return []styles.Hint{
		{Key: "n", Desc: "new"},
		{Key: "enter", Desc: "restore"},
		{Key: "d", Desc: "delete"},
		{Key: "/", Desc: "filter"},
		{Key: "backspace", Desc: "back"},
	}
}

func (m BackupsModel) Title() string { return "Backups" }

func (m BackupsModel) SetLayout(l core.Layout) core.Action {
	m.layout = l
	m.base = m.setCount(len(m.visible()))
	return m
}

func (m BackupsModel) SetFocus(focused bool) core.Action {
	m.isFocus = focused
	return m
}

func (m BackupsModel) SetServer(server *storage.Server) core.Action {
	m.server = server
	m.confirming = nil

	return m.reload()
}

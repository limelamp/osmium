# A map approach for pages to preserve the states of pages

The reason your states reset is because of how Go handles **pass-by-value** in combination with Bubble Tea's update loop.

Here is exactly what is happening in your code:

1. When you initialize your app, you create a copy of `m.dashboard` and assign it to `m.activePage`.
2. When you are on the Dashboard and trigger an action (like pressing "a"), the message is forwarded to `m.activePage`. `m.activePage` returns an **updated copy** of itself, which you assign back to `m.activePage`.
3. **The problem:** You never sync this new updated state back to the `m.dashboard` field! `m.dashboard` remains frozen in the state it was initialized in.
4. When you leave the page and come back, you call `m.switchPage(m.dashboard)`. This pulls that old, frozen `m.dashboard` state and overwrites your `activePage`, effectively "resetting" it.

### The Fix

To fix this, you just need to save the newly updated `m.activePage` state back into its corresponding field in `appModel` after it updates.

Update the bottom of your `Update` function in **`app.go`** to look like this:

```go
	// forwards the same incoming message (including key presses) to the currently active page model
	updated, cmd := m.activePage.Update(msg)
	m.activePage = updated.(Page)

	// NEW: Sync the updated state back to its specific struct field so it doesn't get lost
	switch p := m.activePage.(type) {
	case pages.DashboardModel:
		m.dashboard = p
	case pages.HomeModel:
		m.home = p
	case pages.CreateServerModel:
		m.createServer = p
	case pages.ManageServersModel:
		m.manageServers = p
	case pages.SettingsModel:
		m.settings = p
	}

	return m, cmd
}
```

### Alternative Approach: Map-based Routing (Optional)

If you don't want to maintain a huge `switch` statement every time you add a new page to your app, a very common Bubble Tea pattern is to store your pages in a `map[string]Page` instead of individual struct fields.

It would look something like this:

```go
type appModel struct {
	pages       map[string]Page
	activeKey   string          // e.g. "Dashboard", "Home"
	layout      core.Layout
}

func NewAppModel() appModel {
    home := pages.NewHomeModel()
	return appModel{
		activeKey: "Home",
		pages: map[string]Page{
			"Home":          home,
			"Dashboard":     pages.NewDashboardModel(),
			"CreateServer":  pages.NewCreateServerModel(),
			"ManageServers": pages.NewManageServersModel(),
			"Settings":      pages.NewSettingsModel(),
		},
	}
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ... handling window size and global keys ...

	case core.ChangePageMsg:
		m.activeKey = msg.Target
        // Update layout on the newly selected page
		m.pages[m.activeKey] = m.pages[m.activeKey].SetLayout(m.layout).(Page)
		return m, nil
	}

	// Update the currently active page and save it directly back to the map
	updated, cmd := m.pages[m.activeKey].Update(msg)
	m.pages[m.activeKey] = updated.(Page)

	return m, cmd
}
```

Using the map completely eliminates the sync issue you're facing, since you are reading and writing from the exact same dictionary key every time. Both solutions will work perfectly, so feel free to use whichever one makes more sense to you!

That is a very fair question, and it is great that you are being defensive about your app's architecture. To address the elephant in the room: **a map inside your `appModel` is absolutely not a singleton, and it is a highly sustainable, idiomatic pattern for Bubble Tea applications.**

Let's break down exactly _why_ it's sustainable, the trade-offs involved, and how to make it bulletproof so you don't run into issues later.

### Why this is NOT a Singleton

A singleton is a global, mutable variable (e.g., `var AppState *Model`) that any file in your project can access and modify at any time. Singletons are terrible because they create hidden dependencies, make unit testing impossible, and cause race conditions.

The `map[string]Page` approach I suggested lives **entirely inside your `appModel` struct**. It is passed down sequentially through Bubble Tea's update loop. It strictly adheres to the Elm Architecture (which Bubble Tea is based on): state only changes via pure messages, and everything is isolated to the component tree.

### Struct Fields vs. Map Router

Both approaches (Struct fields vs. Map) are valid in Bubble Tea, but they have different trade-offs as your app grows.

#### 1. The Struct Fields Approach (What you have now)

```go
type appModel struct {
    dashboard pages.DashboardModel
    home      pages.HomeModel
    // ... 20 more fields
}
```

- **Pros:** It is 100% type-safe. The compiler knows exactly what type each page is.
- **Cons:** Massive boilerplate. Every time you add a page, you have to add it to the struct, initialize it in `NewAppModel`, add it to the `ChangePageMsg` switch case, and add it to the state-syncing switch case.

#### 2. The Map Approach (The standard TUI Router)

```go
type appModel struct {
    pages map[string]Page
}
```

- **Pros:** Zero boilerplate routing. You can add 50 pages to your app, and your `app.go` file's `Update` method will never change. It inherently solves the state-reset bug you just experienced.
- **Cons:** You lose compile-time strictness on the _keys_. If you accidentally type `core.RouteTo("Dashbord")` (missing the 'a'), your map will return `nil`, and the app will panic when it tries to call `.Update()` on a nil page.

### How to make the Map approach bulletproof

If you choose the map approach, you should fix its one major flaw (string typos) by using **Constants**. This gives you the best of both worlds: zero routing boilerplate + compile-time safety.

**1. Define your routes as constants:**

```go
// tui/internal/tui/core/messages.go
package core

type Route string

const (
	RouteHome          Route = "Home"
	RouteDashboard     Route = "Dashboard"
	RouteCreateServer  Route = "CreateServer"
	RouteManageServers Route = "ManageServers"
	RouteSettings      Route = "Settings"
)

type ChangePageMsg struct {
	Target Route // Use the strict type here
}

func RouteTo(target Route) tea.Cmd {
	return func() tea.Msg {
		return ChangePageMsg{Target: target}
	}
}
```

**2. Update your appModel to use the Route type:**

```go
type appModel struct {
	pages     map[core.Route]Page
	activeKey core.Route
	layout    core.Layout
}

func NewAppModel() appModel {
	return appModel{
		activeKey: core.RouteHome,
		pages: map[core.Route]Page{
			core.RouteHome:          pages.NewHomeModel(),
			core.RouteDashboard:     pages.NewDashboardModel(),
			core.RouteCreateServer:  pages.NewCreateServerModel(),
			core.RouteManageServers: pages.NewManageServersModel(),
			core.RouteSettings:      pages.NewSettingsModel(),
		},
	}
}
```

### The Verdict

Yes, using a map for routing is incredibly sustainable. In fact, if you look at popular open-source Bubble Tea projects (like [gh-dash](https://github.com/dlvhdr/gh-dash) or tools built by Charm themselves), they almost all use maps or lists for managing dynamic tabs, views, and pages.

If your app will only ever have 4-5 pages, stick to your current **struct fields** and just add the state-sync `switch` statement I provided in the previous answer. If you plan on having many pages or nested sub-pages, the **map with constants** is the cleanest, most scalable way to go.

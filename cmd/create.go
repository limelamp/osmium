package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/config"
	"github.com/limelamp/osmium/internal/shared"
	"github.com/limelamp/osmium/internal/supervisor"
	"github.com/limelamp/osmium/internal/tui/components/steps"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/spf13/cobra"
)

type createFlags struct {
	dir     string
	name    string
	engine  string
	version string
	ram     string
	eula    bool
	start   bool
	from    string
}

var createflags createFlags

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Minecraft server.",
	Long: `Creates a server: downloads the jar for the engine and version you pick,
writes osmium.json, accepts the EULA and registers the result.

Anything given as a flag is taken as answered; whatever is left is asked for,
one prompt at a time. Give every flag and nothing is asked at all, which is what
makes this scriptable.

--from rebuilds a server from a manifest — the osmium.json of an existing
server, or the one 'osmium destroy' kept: the engine, version and memory come
from it, and every mod and plugin it lists is installed into the new server.

Examples:
  osmium create
  osmium create --name "Survival" --engine Paper --version 1.21.1
  osmium create -n Survival -e Fabric -v 1.21.1 --ram 6G --eula --start
  osmium create --from survival.osmium.json --name "Survival II"`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		appConfig, _ := config.LoadConfig()

		manifest, err := readManifest(createflags.from)
		if err != nil {
			return err
		}
		applyManifest(manifest)

		wizardConfig := &steps.Config{
			Dir:      createflags.dir,
			Name:     strings.TrimSpace(createflags.name),
			Category: categoryOf(createflags.engine),
			Software: canonicalEngine(createflags.engine),
			Version:  createflags.version,
			RAM:      createflags.ram,
			Eula:     createflags.eula,
			AutoRun:  createflags.start,
		}

		if wizardConfig.RAM == "" {
			wizardConfig.RAM = appConfig.DefaultRAM
		} else {
			amount, err := steps.NormaliseRAM(wizardConfig.RAM)
			if err != nil {
				return err
			}
			wizardConfig.RAM = amount
		}

		// Only what the flags left unanswered is asked for. A fully specified
		// create runs without a prompt.
		if pending := pendingSteps(wizardConfig, appConfig.ServersDir()); len(pending) > 0 {
			finished, err := runWizard(pending, wizardConfig)
			if err != nil {
				return err
			}
			if !finished {
				fmt.Println("Cancelled. Nothing was created.")
				return nil
			}
		}

		progress := newReporter()
		defer progress.Close()

		server, err := steps.Build(wizardConfig, store, progress.Scope(""))
		if err != nil {
			return err
		}
		progress.Close()
		fmt.Printf("\nCreated %s in %s\n", server.Name, server.Path)

		if err := restoreProjects(manifest, server.Path); err != nil {
			return err
		}

		if wizardConfig.AutoRun {
			if err := supervisor.Start(server.Path, server.Name); err != nil {
				return err
			}
			fmt.Printf("Started %s. Follow it with 'osmium logs -f -s %s'.\n", server.Name, server.Name)
		}
		return nil
	},
}

// readManifest loads the osmium.json a rebuild is based on.
func readManifest(path string) (*config.OsmiumConfig, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest config.OsmiumConfig
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("%s is not an osmium manifest: %w", path, err)
	}
	return &manifest, nil
}

// applyManifest fills in whatever the manifest already answers, leaving flags
// given on the command line to win over it.
func applyManifest(manifest *config.OsmiumConfig) {
	if manifest == nil {
		return
	}

	if createflags.engine == "" {
		createflags.engine = manifest.Loader
	}
	if createflags.version == "" {
		createflags.version = manifest.Version
	}
	if createflags.ram == "" {
		createflags.ram = manifest.AllocatedRAM
	}
}

// restoreProjects installs the mods and plugins a manifest listed into the
// server just built from it, which is what makes a rebuild a rebuild rather
// than an empty server with the same version.
func restoreProjects(manifest *config.OsmiumConfig, target string) error {
	if manifest == nil || (len(manifest.Mods) == 0 && len(manifest.Plugins) == 0) {
		return nil
	}

	built, err := config.ReadConfigIn(target)
	if err != nil {
		return err
	}

	built.Mods, built.Plugins = manifest.Mods, manifest.Plugins
	if err := config.WriteConfigIn(target, built); err != nil {
		return err
	}

	fmt.Printf("\nInstalling %d mods and %d plugins from the manifest…\n",
		len(manifest.Mods), len(manifest.Plugins))

	progress := newReporter()
	defer progress.Close()

	return shared.InstallProjectsFromConfig(progress.Scope(target))
}

// pendingSteps is the wizard reduced to the questions the flags left open.
func pendingSteps(cfg *steps.Config, defaultDir string) []steps.Step {
	var pending []steps.Step

	if cfg.Dir == "" {
		pending = append(pending, steps.NewLocationStep(cfg, defaultDir))
	}
	if cfg.Name == "" {
		pending = append(pending, steps.NewNameStep(cfg))
	}
	if cfg.Software == "" {
		pending = append(pending, steps.NewEngineStep(cfg))
	}
	if cfg.Version == "" {
		pending = append(pending, steps.NewVersionStep(cfg))
	}
	if createflags.ram == "" {
		// The configured default only pre-selects the step; it is still asked,
		// because --ram is what says the question has been answered.
		pending = append(pending, steps.NewMemoryStep(cfg))
	}
	if !cfg.Eula {
		// The EULA cannot be accepted on someone's behalf, so its step is only
		// skipped when --eula said so explicitly.
		pending = append(pending, steps.NewAgreementsStep(cfg))
	}

	// The review is only worth showing when something was actually asked.
	if len(pending) > 0 {
		pending = append(pending, steps.NewReviewStep(cfg))
	}
	return pending
}

// canonicalEngine matches a typed engine name to the one Osmium installs with,
// so "paper" and "Paper" both work.
func canonicalEngine(name string) string {
	if name == "" {
		return ""
	}

	for _, known := range allEngines() {
		if strings.EqualFold(known, name) {
			return known
		}
	}
	return name
}

// categoryOf is the group an engine belongs to, which the manifest records.
func categoryOf(name string) string {
	switch engine := canonicalEngine(name); {
	case engine == "":
		return ""
	case engine == "Vanilla":
		return "Vanilla"
	case contains(pluginEngines(), engine):
		return "Plugin"
	default:
		return "Mod Loader"
	}
}

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&createflags.dir, "dir", "d", "", "Folder to create the server in")
	createCmd.Flags().StringVarP(&createflags.name, "name", "n", "", "Name of the server")
	createCmd.Flags().StringVarP(&createflags.engine, "engine", "e", "", "Server software (Vanilla, Paper, Fabric, …)")
	createCmd.Flags().StringVarP(&createflags.version, "version", "v", "", "Minecraft version")
	createCmd.Flags().StringVar(&createflags.ram, "ram", "", "Memory to allocate, e.g. 6G")
	createCmd.Flags().BoolVar(&createflags.eula, "eula", false, "Accept the Minecraft EULA (https://aka.ms/MinecraftEULA)")
	createCmd.Flags().BoolVar(&createflags.start, "start", false, "Start the server once it is built")
	createCmd.Flags().StringVar(&createflags.from, "from", "", "Rebuild from a manifest (an osmium.json)")

	_ = createCmd.RegisterFlagCompletionFunc("engine", completeLoaders)
	_ = createCmd.RegisterFlagCompletionFunc("version", completeVersions)
	_ = createCmd.MarkFlagDirname("dir")
	_ = createCmd.MarkFlagFilename("from", "json")
}

// wizard runs the create steps one prompt at a time, in the flow of the
// terminal rather than in a full-screen page.
//
// The steps are the very ones the TUI's Create Server page uses: the two
// wizards ask the same questions, validate them the same way and produce the
// same configuration, because there is only one implementation of each step.
type wizard struct {
	config *steps.Config
	steps  []steps.Step
	active int

	width    int
	finished bool
}

// promptHeight is how many rows a step's body may use inline. It is enough for
// the version grid to show a useful number of rows without taking over the
// terminal the way the full-screen wizard does.
const promptHeight = 12

// runWizard asks the pending questions and reports whether they were all
// answered, as opposed to cancelled part way through.
func runWizard(pending []steps.Step, cfg *steps.Config) (bool, error) {
	model := wizard{config: cfg, steps: pending, width: 80}

	program := tea.NewProgram(model)
	final, err := program.Run()
	if err != nil {
		return false, err
	}

	answered, ok := final.(wizard)
	if !ok {
		return false, fmt.Errorf("the wizard ended as %T", final)
	}
	return answered.finished, nil
}

func (w wizard) Init() tea.Cmd {
	return w.steps[0].Init()
}

func (w wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w.width = msg.Width
		return w.layout(), nil

	case steps.NextMsg:
		if w.active == len(w.steps)-1 {
			w.finished = true
			return w, tea.Quit
		}
		w.active++
		return w.layout(), w.steps[w.active].Init()

	case steps.PrevMsg:
		if w.active > 0 {
			w.active--
		}
		return w.layout(), nil

	case core.ChangePageMsg:
		// A step asking to leave for a page has nowhere to go out here, so it
		// means the same thing cancelling does.
		return w, tea.Quit

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return w, tea.Quit
		}
	}

	updated, cmd := w.steps[w.active].Update(msg)
	if step, ok := updated.(steps.Step); ok {
		w.steps[w.active] = step
	}
	return w, cmd
}

// layout re-flows the current step for the terminal width.
func (w wizard) layout() wizard {
	w.steps[w.active] = w.steps[w.active].SetLayout(core.Layout{
		Width:  max(w.width-2, 20),
		Height: promptHeight,
	})
	return w
}

func (w wizard) View() tea.View {
	step := w.steps[w.active]

	heading := styles.StepHeading(w.active+1, step.Title(), step.Subtitle())
	hints := styles.Hints(w.width,
		styles.Hint{Key: "↑/↓/←/→", Desc: "navigate"},
		styles.Hint{Key: "enter", Desc: "select"},
		styles.Hint{Key: "ctrl+c", Desc: "cancel"},
	)

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		heading,
		"",
		step.View().Content,
		"",
		hints,
	)

	// No alt screen: the prompt scrolls with the shell's history like any other
	// command's output, which is what makes this read as a CLI wizard.
	return tea.NewView(lipgloss.NewStyle().PaddingLeft(1).Render(body))
}

// allEngines is every server software the wizard offers.
func allEngines() []string {
	engines, _ := completeLoaders(nil, nil, "")
	return engines
}

// pluginEngines are the plugin-based servers, which the manifest groups apart
// from the mod loaders.
func pluginEngines() []string {
	return []string{"Paper", "Purpur", "Spigot", "Bukkit"}
}

func contains(list []string, wanted string) bool {
	for _, item := range list {
		if item == wanted {
			return true
		}
	}
	return false
}

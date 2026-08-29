package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// completionCmd replaces Cobra's generated one so that installing the script
// can be a subcommand of it. Printing to stdout still works exactly as before;
// what is new is that Osmium can put the script where the shell will find it,
// because generating a script nobody installs is why tab did nothing.
var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Shell completion for Osmium.",
	Long: `Osmium completes server names, installed mods and plugins, Minecraft versions
and engines as you type.

Run 'osmium completion install' once and open a new shell. The per-shell
subcommands print the script instead, for placing it yourself.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// completionInstallCmd writes the script where the running shell looks for it.
var completionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install completion for your shell.",
	Long: `Writes the completion script into the directory your shell reads completions
from, and tells you what it did.

The shell is taken from $SHELL unless one is named:
  osmium completion install
  osmium completion install zsh`,
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := currentShell()
		if len(args) == 1 {
			shell = args[0]
		}

		// PowerShell has no directory it loads completions from: they are added
		// to the session by the profile script. There is nothing to install, so
		// it is told rather than done.
		if shell == "powershell" || shell == "pwsh" {
			fmt.Println("PowerShell loads completions from your profile rather than from a directory.")
			fmt.Println("Add this line to the file that $PROFILE names:")
			fmt.Println()
			fmt.Println("  osmium completion powershell | Out-String | Invoke-Expression")
			return nil
		}

		target, err := completionPath(shell)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}

		file, err := os.Create(target)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		if err := writeCompletion(shell, file); err != nil {
			return err
		}

		fmt.Printf("Installed %s completion to %s\n", shell, target)
		fmt.Println("Open a new shell and press tab after 'osmium '.")

		if shell == "zsh" {
			fmt.Printf("\nZsh only reads completions from directories in its fpath. If nothing\n"+
				"completes, add this line to ~/.zshrc:\n\n  fpath=(%s $fpath)\n",
				filepath.Dir(target))
		}
		return nil
	},
}

// The per-shell subcommands print the script, for anyone placing it themselves.
var completionShellCmds = []struct {
	shell string
	short string
}{
	{"bash", "Print the bash completion script."},
	{"zsh", "Print the zsh completion script."},
	{"fish", "Print the fish completion script."},
	{"powershell", "Print the PowerShell completion script."},
}

// writeCompletion generates the script for a shell.
func writeCompletion(shell string, out *os.File) error {
	switch shell {
	case "bash":
		return rootCmd.GenBashCompletionV2(out, true)
	case "zsh":
		return rootCmd.GenZshCompletion(out)
	case "fish":
		return rootCmd.GenFishCompletion(out, true)
	case "powershell", "pwsh":
		return rootCmd.GenPowerShellCompletionWithDesc(out)
	}
	return fmt.Errorf("unsupported shell %q - bash, zsh, fish and powershell are supported", shell)
}

// completionPath is where a shell loads a user's completions from.
func completionPath(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}

	switch shell {
	case "bash":
		// bash-completion v2 loads this directory on demand, with no line to
		// add to any config file.
		return filepath.Join(data, "bash-completion", "completions", "osmium"), nil
	case "fish":
		config := os.Getenv("XDG_CONFIG_HOME")
		if config == "" {
			config = filepath.Join(home, ".config")
		}
		return filepath.Join(config, "fish", "completions", "osmium.fish"), nil
	case "zsh":
		return filepath.Join(data, "zsh", "site-functions", "_osmium"), nil
	}

	return "", fmt.Errorf("unsupported shell %q - bash, zsh, fish and powershell are supported", shell)
}

// currentShell is the shell that ran Osmium, as far as the environment says.
// Windows sets no SHELL, so PowerShell is the assumption there.
func currentShell() string {
	shell := filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	if shell == "" || shell == "." {
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return ""
	}
	return strings.TrimSuffix(shell, ".exe")
}

func init() {
	// Cobra's own completion command is replaced by this one, which does the
	// same thing plus the installing.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionInstallCmd)

	for _, shell := range completionShellCmds {
		completionCmd.AddCommand(&cobra.Command{
			Use:   shell.shell,
			Short: shell.short,
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return writeCompletion(cmd.Name(), os.Stdout)
			},
		})
	}
}

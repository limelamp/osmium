package cmd

import (
	"fmt"

	"github.com/limelamp/osmium/internal/cache"
	"github.com/limelamp/osmium/internal/pack"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Show the download cache.",
	Long: `Says where Osmium keeps the files it has downloaded, and how much room
they take.

Files are kept by their content, so the same mod wanted by a second server,
by a reinstall, or by a migration back to a version installed last month is
already there and costs no network at all. Nothing in the cache is needed to
run a server; emptying it only means the next install downloads again.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := cache.Open()
		if err != nil {
			return err
		}

		files, bytes, err := store.Size()
		if err != nil {
			return err
		}

		fmt.Println(store.Root())
		fmt.Printf("%d files   %s\n", files, pack.FormatBytes(bytes))
		return nil
	},
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Empty the download cache.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := cache.Open()
		if err != nil {
			return err
		}

		files, bytes, err := store.Clean()
		if err != nil {
			return err
		}

		fmt.Printf("Removed %d files, %s.\n", files, pack.FormatBytes(bytes))
		return nil
	},
}

func init() {
	cacheCmd.AddCommand(cacheCleanCmd)
	rootCmd.AddCommand(cacheCmd)
}

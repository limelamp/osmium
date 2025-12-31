package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var shouldSumBeInt bool
var addCmd = &cobra.Command{
	Use:     "add",
	Aliases: []string{"addition"},
	Short:   "Add 2 numbers",
	Long:    "Carry out addition operation on 2 numbers",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Addition of %s and %s = %s.\n\n", args[0], args[1], Add(args[0], args[1], shouldSumBeInt))
	},
}

func init() {
	addCmd.Flags().BoolVarP(&shouldSumBeInt, "int", "i", false, "Outputs the result as an integer")
	rootCmd.AddCommand(addCmd)
}

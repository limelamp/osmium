package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var shouldDifferenceBeInt bool
var subtractCmd = &cobra.Command{
	Use:     "subtract",
	Aliases: []string{"sub"},
	Short:   "Subtract a number from another",
	Long:    "Carry out subtraction operation on 2 integers",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Subtraction of %s from %s = %s.\n\n", args[1], args[0], Subtract(args[0], args[1], shouldDifferenceBeInt))
	},
}

func init() {
	subtractCmd.Flags().BoolVarP(&shouldDifferenceBeInt, "int", "i", false, "Outputs the result as an integer")
	rootCmd.AddCommand(subtractCmd)
}

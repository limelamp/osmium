/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec [minecraft command]",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		conn, err := net.Dial("tcp", "127.0.0.1:59072")
		if err != nil {
			fmt.Println("Server is not running (couldn't connect to socket).")
			return
		}
		defer conn.Close()

		// Send the 'message' to the background daemon a.k.a. the server
		fmt.Fprintln(conn, strings.Join(args, " "))
		fmt.Println(strings.Join(args, " "))
	},
}

func init() {
	rootCmd.AddCommand(execCmd)

}

package cmd

import (
	"github.com/erdemkosk/gofi/internal"
	"github.com/erdemkosk/gofi/internal/command"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server and client",
	Long:  `This command starts the UDP server and client.`,
	Run:   command.CommandFactory(internal.START).Execute,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

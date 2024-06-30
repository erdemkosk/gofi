package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gofi",
	Short: "Gofi is a CLI tool for managing UDP server and client",
	Long:  `Gofi is a CLI tool built with Cobra to manage UDP server and client.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {

}

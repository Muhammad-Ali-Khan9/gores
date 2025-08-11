package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gores",
	Short: "Go Microservice Boilerplate Generator CLI",
	Long:  "A CLI tool to generate Go microservice boilerplate code with controllers, services, entities, routers, and more.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
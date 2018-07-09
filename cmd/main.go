package main

import (
	"fmt"
	"os"

	"github.com/solo-io/gloo-connect/pkg/cmd"
	"github.com/solo-io/gloo-connect/pkg/runner"
)

func main() {
	rc := runner.RunConfig{}
	rootCmd := cmd.Cmd(&rc)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

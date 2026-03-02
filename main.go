package main

import (
	"os"

	"github.com/jmvrbanac/slackseek/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

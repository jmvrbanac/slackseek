package main

import (
	"os"

	"github.com/jmvrbanac/slackseek/cmd"
)

// version is set at build time via -ldflags "-X main.version=vX.Y.Z".
var version = "dev"

func main() {
	if err := cmd.Execute(version); err != nil {
		os.Exit(1)
	}
}

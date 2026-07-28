package main

import (
	"log"
	"os"

	"github.com/daptin/daptin-cli/cmd"
)

var version = "dev"

func main() {
	// Init logger early at warn level; --debug upgrades to debug in Before hook
	cmd.InitLogger(false)

	app := cmd.NewAppFromConfig(version)
	if err := app.Run(cmd.ReorderArgs(os.Args)); err != nil {
		log.Fatal(err)
	}
}

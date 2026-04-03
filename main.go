package main

import (
	"log"
	"os"

	"github.com/daptin/daptin-cli/cmd"
	"github.com/daptin/daptin-cli/config"
)

var version = "dev"

func main() {
	cfgPath := config.ResolvePath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	app := cmd.NewApp(&cfg, version)
	if err := app.Run(cmd.ReorderArgs(os.Args)); err != nil {
		log.Fatal(err)
	}
}

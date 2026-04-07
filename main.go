package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/daptin/daptin-cli/cmd"
	"github.com/daptin/daptin-cli/config"
)

var version = "dev"

func main() {
	// Init logger early at warn level; --debug upgrades to debug in Before hook
	cmd.InitLogger(false)

	cfgPath := config.ResolvePath()
	slog.Debug("resolved config path", "path", cfgPath)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	slog.Debug("starting daptin-cli", "version", version, "config_path", cfgPath)

	app := cmd.NewApp(&cfg, version)
	if err := app.Run(cmd.ReorderArgs(os.Args)); err != nil {
		log.Fatal(err)
	}
}

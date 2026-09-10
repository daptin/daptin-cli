package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/daptin/daptin-cli/cmd"
)

var version = "dev"

func main() {
	// Init logger early at warn level; --debug upgrades to debug in Before hook
	cmd.InitLogger(false)

	app := cmd.NewAppFromConfig(version)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.RunContext(ctx, cmd.ReorderArgs(os.Args)); err != nil {
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		exitCode := 1
		if exitErr, ok := err.(interface{ ExitCode() int }); ok {
			exitCode = exitErr.ExitCode()
		}
		os.Exit(exitCode)
	}
}

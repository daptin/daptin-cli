package cmd

import (
	"github.com/urfave/cli/v2"
)

func healthCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "health",
		Usage: "Check Daptin API readiness",
		Action: func(c *cli.Context) error {
			result, err := appCtx.Client.Health()
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(result)
		},
	}
}

func statisticsCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "statistics",
		Usage: "Show secret-safe Daptin runtime statistics",
		Action: func(c *cli.Context) error {
			result, err := appCtx.Client.Statistics()
			if err != nil {
				return err
			}
			return appCtx.Renderer.RenderObject(result)
		},
	}
}

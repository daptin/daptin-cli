package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/daptin/daptin-cli/config"
	"github.com/urfave/cli/v2"
)

func contextCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "context",
		Usage: "Manage server contexts",
		Subcommands: []*cli.Command{
			{
				Name:      "set",
				Usage:     "Set the active context",
				ArgsUsage: "<name>",
				Action: func(c *cli.Context) error {
					name := c.Args().Get(0)
					if name == "" {
						return fmt.Errorf("context name required")
					}
					slog.Info("context set", "name", name)
					if err := appCtx.Config.SetContext(name); err != nil {
						return err
					}
					return appCtx.Config.Save()
				},
			},
			{
				Name:      "add",
				Usage:     "Add a new context",
				ArgsUsage: "<name> <endpoint>",
				Action: func(c *cli.Context) error {
					name := c.Args().Get(0)
					endpoint := c.Args().Get(1)
					if name == "" || endpoint == "" {
						return fmt.Errorf("usage: context add <name> <endpoint>")
					}
					slog.Info("context add", "name", name, "endpoint", endpoint)
					appCtx.Config.UpsertHost(config.HostEndpoint{
						Name:     name,
						Endpoint: endpoint,
					})
					return appCtx.Config.Save()
				},
			},
			{
				Name:  "list",
				Usage: "List all contexts",
				Action: func(c *cli.Context) error {
					slog.Info("context list", "count", len(appCtx.Config.Hosts))
					if len(appCtx.Config.Hosts) == 0 {
						fmt.Println("No contexts configured")
						return nil
					}
					now := time.Now()
					for _, h := range appCtx.Config.Hosts {
						marker := "  "
						if h.Name == appCtx.Config.CurrentContext {
							marker = "* "
						}
						authLabel := contextAuthLabel(h.Token, now)
						slog.Debug("context entry", "name", h.Name, "endpoint", h.Endpoint, "token_present", h.Token != "")
						fmt.Printf("%s%s  %s%s\n", marker, h.Name, h.Endpoint, authLabel)
					}
					return nil
				},
			},
		},
	}
}

func contextAuthLabel(token string, now time.Time) string {
	if token == "" {
		return ""
	}

	usable, status, _ := config.IsTokenUsable(token, now)
	if status == config.TokenExpiryExpired {
		return " (expired)"
	}
	if usable {
		return " (authenticated)"
	}
	return ""
}
